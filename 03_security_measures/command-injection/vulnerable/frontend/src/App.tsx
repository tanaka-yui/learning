import { useState } from "react";

const API_URL = import.meta.env.VITE_API_URL ?? "http://localhost:8090";

interface CommandResult {
  output?: string;
  error?: string;
}

function App() {
  const [host, setHost] = useState("");
  const [result, setResult] = useState("");
  const [error, setError] = useState("");

  const executeCommand = async (endpoint: string) => {
    try {
      setError("");
      setResult("");
      const response = await fetch(`${API_URL}/${endpoint}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ host }),
      });
      const data: CommandResult = await response.json();
      if (data.error) {
        setError(data.error);
      } else if (data.output) {
        setResult(data.output);
      }
    } catch (e) {
      if (e instanceof Error) {
        setError(e.message);
      }
    }
  };

  return (
    <div className="container">
      <div className="banner banner-danger">
        このアプリケーションは学習目的で意図的に脆弱に作られています
      </div>
      <h1>コマンドインジェクション - 脆弱版</h1>

      <div className="input-form">
        <input
          type="text"
          value={host}
          onChange={(e) => setHost(e.target.value)}
          placeholder="ホスト名を入力 (例: localhost)"
          className="host-input"
        />
        <button
          type="button"
          className="action-button btn-lookup"
          onClick={() => void executeCommand("lookup")}
        >
          DNS Lookup
        </button>
        <button
          type="button"
          className="action-button btn-ping"
          onClick={() => void executeCommand("ping")}
        >
          Ping
        </button>
      </div>

      {error && <p className="error">{error}</p>}

      <div className="terminal-output">
        {result ? (
          <pre><code>{result}</code></pre>
        ) : (
          <p className="terminal-placeholder">
            実行結果がここに表示されます
          </p>
        )}
      </div>

      <div className="attack-examples">
        <h2>攻撃ペイロードの例</h2>
        <div className="example">
          <code>localhost; cat /etc/passwd</code>
          <span> -- ファイル読み取り</span>
        </div>
        <div className="example">
          <code>localhost; whoami</code>
          <span> -- ユーザー確認</span>
        </div>
        <div className="example">
          <code>localhost && ls /</code>
          <span> -- ディレクトリ一覧</span>
        </div>
        <div className="example">
          <code>localhost | echo HACKED</code>
          <span> -- パイプ</span>
        </div>
      </div>
    </div>
  );
}

export default App;
