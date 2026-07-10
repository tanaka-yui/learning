# 03_activemq: ブローカー常駐型 MQ の queue と topic

`01_concepts.md` は queue を「ブローカーが配って消す」モデルと位置づけ、その代表例として SQS と並べて ActiveMQ を挙げた。本ドキュメントでは ActiveMQ が同じ queue 系列でありながら SQS と何が違うのか — 自分でブローカーを飼う運用形態、STOMP という標準プロトコル、そして queue / topic という 2 つの配信モデルを同一ブローカー上で切り替えられる柔軟さ — を `apps/activemq/` の実コードと `make demo-activemq` / `make demo-activemq-topic` の実行結果に沿って掘り下げる。at-least-once や冪等性、fan-out といった用語は `01_concepts.md` の定義をそのまま使うので、ここでは再定義しない。

---

## 1. イントロ — ブローカー常駐型 MQ とは

SQS は API を叩くだけで、こちらが管理するプロセスは 1 つもなかった。ActiveMQ はその対極にある。JMS（Java Message Service）に由来するブローカー常駐型の MQ であり、`apache/activemq-classic` イメージのプロセスを自分で起動し、生死・ポート・認証情報まで自分で面倒を見る（[docker-compose.yml:11](../docker-compose.yml#L11)-[18](../docker-compose.yml#L18)）。本ハンズオンの compose はこのブローカーの STOMP ポート `61613` と管理 Web UI ポート `8161` の 2 つだけをホストに公開している（[docker-compose.yml:14](../docker-compose.yml#L14)-[15](../docker-compose.yml#L15)）。

SQS との一番わかりやすい違いは、ブローカーの中身が「見える」ことである。`http://localhost:8161` の管理 UI に `admin`/`admin`（これはイメージ既定の資格情報であり、[docker-compose.yml:17](../docker-compose.yml#L17)-[18](../docker-compose.yml#L18)の `ACTIVEMQ_CONNECTION_USER`/`PASSWORD` はブローカー接続〈STOMP 等〉用の資格情報で、値がたまたま一致しているだけである）でログインすると、キューごとの滞留数・消費速度・接続中のコンシューマ数がダッシュボードとして見える。SQS では CloudWatch メトリクスやポーリングでしか窺えなかった「今キューに何通溜まっているか」が、ブローカーを自分で持つことの直接的な見返りとして UI 上に現れる。この代わりに、ブローカーの可用性・スケール・パッチ適用は AWS ではなく利用者自身の責任になる。

なお `apache/activemq-classic` のコンテナはイメージとして STOMP（61613）・管理 UI（8161）以外にも OpenWire（61616）・AMQP（5672）・MQTT（1883）・JMX（1099）のポートを内部で公開しているが、本 compose では STOMP と管理 UI 以外は外部に出していない。この「1 つのブローカーが複数プロトコルの入口を持つ」という性質はセクション 3 で扱う。

本ハンズオンの CLI は `produce` / `consume` の 2 サブコマンドで構成され（[apps/activemq/main.go:52](../apps/activemq/main.go#L52)-[57](../apps/activemq/main.go#L57)）、ブローカーへの接続に失敗した場合は「`make up` を先に実行してください」という具体的なヒント付きのエラーを返す（[apps/activemq/main.go:40](../apps/activemq/main.go#L40)-[42](../apps/activemq/main.go#L42)）。SQS では「API エンドポイントが応答するかどうか」を気にすることはまずなかったが、ブローカーを自分で飼うということは、こうした「起動を待つ」「繋がらない理由を利用者に教える」といった面倒もアプリケーション側のコードで引き受けるということでもある。

---

## 2. queue と topic — 同じブローカー、宛先名だけの切り替え

ActiveMQ は同一ブローカー上に **queue**（競合コンシューマ）と **topic**（fan-out）という 2 つの配信モデルを持ち、その切り替えは宛先の文字列だけで決まる。

```go
// destination で queue（競合コンシューマ）と topic（fan-out）を切り替えるのが STOMP 流。
func destination(topic bool) string {
	if topic {
		return "/topic/orders"
	}
	return "/queue/orders"
}
```
（[apps/activemq/main.go:28](../apps/activemq/main.go#L28)-[34](../apps/activemq/main.go#L34)）

producer・consumer とも同じこの関数で宛先を決めており（[apps/activemq/producer.go:21](../apps/activemq/producer.go#L21), [apps/activemq/consumer.go:24](../apps/activemq/consumer.go#L24)）、`--topic` フラグ 1 つで挙動が変わる（[apps/activemq/consumer.go:14](../apps/activemq/consumer.go#L14)）。ブローカーの設定や別サービスを用意する必要はなく、クライアントが宛先名として `/queue/...` と書くか `/topic/...` と書くかだけで配信モデルが決まる。

**`/queue/orders`** は `01_concepts.md` で定義した「配って消す」競合コンシューマそのものである。`make demo-activemq`（[Makefile:28](../Makefile#L28)-[30](../Makefile#L30)）は先に 5 通を `produce` し、そのあとで `consume` を起動する順序になっているが、これは queue が **produce の時点でまだ誰も consume していなくても、ブローカーがメッセージを保持し続ける** ためである。実行結果は次の通りで、先に送った 5 通がすべて後からの consume で受信できる（実測）。

```
go run ./apps/activemq produce -n 5
sent to /queue/orders: {"id":"order-0001", ...}
...(5 通)
go run ./apps/activemq consume -max 5
received from /queue/orders: {"id":"order-0001", ...}
...(5 通、producer 終了後に起動した consumer がすべて受信)
```

**`/topic/orders`** は fan-out、つまり購読しているコンシューマ全員に同じメッセージが配られるモデルである。ただし topic には queue のような「保持」がない。**購読していない間に発行されたメッセージは、そのコンシューマにとって永遠に失われる**。`make demo-activemq-topic`（[Makefile:33](../Makefile#L33)-[39](../Makefile#L39)）が「2 つの consumer を先に起動し、2 秒待ってから produce する」という順序を徹底しているのはこのためであり、もし produce を先に実行してしまうと、まだ誰も購読していない topic にメッセージを送ることになり、後から起動した consumer には何も届かない。実行すると次の結果が得られる（実測）。

```
sent to /topic/orders: {"id":"order-0001", ...}
sent to /topic/orders: {"id":"order-0002", ...}
sent to /topic/orders: {"id":"order-0003", ...}
received from /topic/orders: {"id":"order-0001", ...}
received from /topic/orders: {"id":"order-0001", ...}
received from /topic/orders: {"id":"order-0002", ...}
received from /topic/orders: {"id":"order-0003", ...}
received from /topic/orders: {"id":"order-0002", ...}
received from /topic/orders: {"id":"order-0003", ...}
```

送った 3 通に対し受信ログが 6 行あることが fan-out の証拠であり、2 つの consumer プロセスがそれぞれ 3 通ずつ、同じ 3 通を独立に受け取っている（`order-0001`〜`0003` がそれぞれ 2 回ずつ出現する）。なお、この「先に接続していた購読者にしか届かない」という制約は JMS の **durable subscription**（購読者を ID で登録し、切断中に発行されたメッセージも後から受け取れるようにする仕組み）を使えば緩和できるが、本ハンズオンの consumer はその場限りの非永続購読であり、この節で見た「先に consumer、後から produce」という順序の制約をそのまま受ける。

---

## 3. STOMP / AMQP / OpenWire — 標準プロトコルという可搬性

ActiveMQ を SQS と隔てるもう 1 つの軸は、通信プロトコルが標準化されているか、AWS 独自かという点である。

本ハンズオンの `dial()` は生の STOMP（Streaming Text Oriented Messaging Protocol）を TCP 上で話しているだけである。

```go
func dial() (*stomp.Conn, error) {
	conn, err := stomp.Dial("tcp", "localhost:61613",
		stomp.ConnOpt.Login("admin", "admin"),
	)
```
（[apps/activemq/main.go:36](../apps/activemq/main.go#L36)-[38](../apps/activemq/main.go#L38)）

STOMP は仕様が公開されたテキストベースのプロトコルであり、`go-stomp/v3` のような Go クライアントに限らず、Python・Java・Ruby・.NET など言語ごとに実装が存在する。しかも STOMP を話せるブローカーは ActiveMQ だけではなく、Artemis（ActiveMQ の後継エンジン）や RabbitMQ も STOMP プラグインで受け付けるため、クライアントコードを書き換えずにブローカーだけ差し替える、といった移行が現実的な選択肢になる。さらに ActiveMQ 自体も STOMP 専用ではなく、同じブローカープロセスが OpenWire（ActiveMQ のネイティブプロトコル、61616 番）や AMQP（5672 番）でも同時に待ち受けており、STOMP クライアントと OpenWire クライアントが同じキューに同時接続する、という構成も組める。

これは SQS の `SendMessage` / `ReceiveMessage` という AWS 独自の HTTPS API（`02_sqs.md` セクション 1）とは対照的である。SQS の API は AWS SDK を経由して叩く前提であり、標準化されたワイヤプロトコルとして公開されているわけではない。したがって SQS 向けに書いたコードをそのまま他ベンダーの queue サービスへ移植することはできず、移行は SDK 呼び出しの書き換えを伴う。「プロトコルが標準化されている＝クライアント言語もブローカー実装も自由に選べる」ことは、ActiveMQ のようなセルフホスト型 MQ が運用の手間と引き換えに得ている可搬性である。

---

## 4. ACK モードと永続化 — 何が確認され、何が消える前提か

`02_sqs.md` セクション 3 では、SQS の `ReceiveMessage` と `DeleteMessage` が別の API 呼び出しに分離されていることが at-least-once の実装そのものだと述べた。ActiveMQ の STOMP でも同じ発想が ACK モードとして現れるが、本ハンズオンの consumer が使っているのは **最も緩いモード** であることに注意が要る。

```go
sub, err := conn.Subscribe(dest, stomp.AckAuto)
```
（[apps/activemq/consumer.go:25](../apps/activemq/consumer.go#L25)）

`stomp.AckAuto` は「メッセージがコンシューマに配送された時点で、ブローカー側は即座に ACK 済みとみなす」自動確認モードである。アプリケーションが `msg.Body` を受け取った直後にプロセスがクラッシュしても、ブローカーはすでにそのメッセージを配送済み・処理済み扱いにしているため、再配信は起こらない。つまり **本ハンズオンの consumer は、受信後・業務処理完了前にクラッシュするとそのメッセージを失いうる** ——これは SQS の「Delete を明示的に呼ぶまで消えない」設計とは逆の性質であり、この demo コードを「ActiveMQ は at-least-once を保証する」根拠として引用してはならない。at-least-once に相当する再配信を得たいなら `stomp.AckClient` や `stomp.AckClientIndividual` を選び、業務処理が完了してから明示的に `conn.Ack(msg)` を呼ぶ設計に変える必要がある。この場合、ACK を呼ぶ前にコンシューマが落ちればブローカーは再接続後にそのメッセージを再配信する。

もう 1 つ、生存性に関わる軸が永続化（persistence）である。STOMP の SEND フレームには `persistent: true` ヘッダーを付けられ、これを付けたメッセージだけがディスク上のストア（既定では KahaDB）に書き込まれ、ブローカー自体が再起動・クラッシュしても残る。逆にこのヘッダーを付けなければ、ActiveMQ の STOMP アダプタは既定で **非永続**（メモリ上のみ）としてメッセージを扱う——これは JMS の既定（永続）とは逆であることに注意したい。本ハンズオンの producer は `conn.Send` にオプションを一切渡していない。

```go
if err := conn.Send(dest, "application/json", body); err != nil {
```
（[apps/activemq/producer.go:24](../apps/activemq/producer.go#L24)）

したがってこのハンズオンで送っているメッセージは非永続であり、`make demo-activemq` が動く理由は「ブローカーがプロセスとして生き続けている間、メモリ上でキューに保持している」だけであって、ブローカーを途中で再起動すればそのメッセージは消える。ジョブの重要度に応じて `persistent:true` を明示するかどうかを選ぶのが実運用での使い分けになる。

---

## 5. SQS との使い分け

ActiveMQ と SQS は同じ queue のデータモデルに立ちながら、運用形態・課金・プロトコル・順序・スケールの前提が大きく異なる。

| 観点 | SQS | ActiveMQ |
|------|-----|----------|
| 運用 | フルマネージド。ブローカーというプロセスの存在を意識しない | 自前でプロセスを運用（本ハンズオンは docker-compose）。AWS 上でマネージドにしたい場合は Amazon MQ（ActiveMQ / RabbitMQ エンジンをインスタンス単位で提供） |
| 課金 | リクエスト課金（`SendMessage` / `ReceiveMessage` などの API 呼び出し数） | インスタンス時間課金。ブローカーを動かすホストが稼働している限り、トラフィックの有無に関わらず費用が発生する |
| プロトコル可搬性 | AWS 独自 HTTPS API（SDK 経由）。他ベンダーへの移植は書き換え前提 | STOMP / AMQP / OpenWire / MQTT を標準サポート。Artemis や RabbitMQ など他ブローカーへの移行やクライアント言語の自由な選択が可能 |
| 順序 | standard は無保証、FIFO は `MessageGroupId` 単位で保証（`02_sqs.md` セクション 2） | queue に対して単一コンシューマなら送信順が保たれるが、複数コンシューマにぶら下げると SQS standard と同様に順序は崩れうる |
| スループット天井 | 実質無制限（AWS 側がバックエンドをスケール） | 単一ブローカーの CPU / ディスク I/O に張り付く。それを超えるにはブローカーのクラスタ化など運用側の設計が必要になる |

**AWS 内で完結し、queue 相当の機能があれば足りるなら SQS が第一候補である。** サーバーの面倒を見る必要がなく、課金もトラフィックに比例するため、スモールスタートや AWS ネイティブな構成と相性がよい。一方で **JMS 資産を引き継ぎたい、オンプレミスで運用する必要がある、STOMP / AMQP / OpenWire のような標準プロトコルでベンダーロックインを避けたい** といった要件があれば、ActiveMQ（あるいはそれを AWS がマネージドで提供する Amazon MQ）が選択肢になる。topic による fan-out が標準機能として最初から使えることも、SQS だけでは（SNS を組み合わせない限り）持てない ActiveMQ 固有の強みである。

---

## 6. make demo-activemq / demo-activemq-topic の実行手順と観察ポイント

スタックが起動していない場合は `make up` を先に実行する。その後、queue の基本挙動は次で確認する。

```bash
make demo-activemq
```

観察ポイントは、producer が終了してから consumer を起動しているにもかかわらず 5 通すべてが受信できることである。これは queue がメッセージをブローカー側に保持しているためであり、SQS で言えば「produce だけ先に済ませてもキューに残っている」ことに相当する（実測結果はセクション 2 に掲載）。

topic の fan-out は次で確認する。

```bash
make demo-activemq-topic
```

このターゲットは 2 つの `consume --topic` をバックグラウンドで先に起動し、2 秒待ってから `produce --topic -n 3` を実行し、最後に両方の consumer の終了コードを `wait` で回収して非ゼロなら失敗として検知する（[Makefile:34](../Makefile#L34)-[39](../Makefile#L39)）。観察ポイントは 2 つある。1 つ目は send が 3 行に対し received が 6 行になること——2 つの独立した consumer がそれぞれ同じ 3 通を受け取った証拠であり、queue の競合コンシューマとは違って「1 通を取り合う」のではなく「全員に配る」ことが数字として確認できる。2 つ目は、もし produce を consumer より先に実行する構成に変えてしまうと received が 0 行になる、という点である（本ハンズオンでは Makefile がその順序ミスを起こさないように `sleep 2` を挟んでいるが、実務でも「購読開始前のメッセージは届かない」という topic の制約は必ず設計に織り込む必要がある）。両方の実行結果はセクション 2 に掲載した実測ログの通りである。
