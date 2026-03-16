import { useState, type FormEvent } from "react";

const API_URL = import.meta.env.VITE_API_URL ?? "http://localhost:8086";

const COMMON_PASSWORDS = [
  "password",
  "123456",
  "password123",
  "admin",
  "letmein",
  "welcome",
  "monkey",
  "dragon",
  "master",
  "qwerty",
  "login",
  "abc123",
  "passw0rd",
  "shadow",
  "123123",
  "654321",
  "superman",
  "michael",
  "access",
  "trustno1",
];

interface UserInfo {
  id: number;
  username: string;
  password: string;
}

interface AdminResponse {
  message: string;
  username: string;
}

interface LoginResponse {
  message: string;
  username: string;
}

interface AttemptResult {
  index: number;
  password: string;
  status: number;
  success: boolean;
}

function App() {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [loggedIn, setLoggedIn] = useState(false);
  const [userInfo, setUserInfo] = useState<UserInfo | null>(null);
  const [adminMessage, setAdminMessage] = useState("");
  const [error, setError] = useState("");
  const [bruteForceRunning, setBruteForceRunning] = useState(false);
  const [attemptResults, setAttemptResults] = useState<AttemptResult[]>([]);

  const handleLogin = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setError("");

    try {
      const response = await fetch(`${API_URL}/login`, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username, password }),
      });

      if (!response.ok) {
        const text = await response.text();
        setError(text || "ログインに失敗しました");
        return;
      }

      const data: LoginResponse = await response.json();
      setLoggedIn(true);
      setUsername(data.username);
      await fetchUserInfo();
      await fetchAdminPage();
    } catch (err) {
      if (err instanceof Error) {
        setError(err.message);
      }
    }
  };

  const fetchUserInfo = async () => {
    try {
      const response = await fetch(`${API_URL}/me`, {
        credentials: "include",
      });
      if (response.ok) {
        const data: UserInfo = await response.json();
        setUserInfo(data);
      }
    } catch {
      // ignore
    }
  };

  const fetchAdminPage = async () => {
    try {
      const response = await fetch(`${API_URL}/admin`, {
        credentials: "include",
      });
      if (response.ok) {
        const data: AdminResponse = await response.json();
        setAdminMessage(data.message);
      }
    } catch {
      // ignore
    }
  };

  const handleLogout = async () => {
    try {
      await fetch(`${API_URL}/logout`, {
        method: "POST",
        credentials: "include",
      });
    } catch {
      // ignore
    }
    setLoggedIn(false);
    setUserInfo(null);
    setAdminMessage("");
    setAttemptResults([]);
    setUsername("");
    setPassword("");
  };

  const handleBruteForce = async () => {
    setBruteForceRunning(true);
    setAttemptResults([]);

    const results: AttemptResult[] = [];

    for (let i = 0; i < COMMON_PASSWORDS.length; i++) {
      const pw = COMMON_PASSWORDS[i];
      if (!pw) continue;

      try {
        const response = await fetch(`${API_URL}/login`, {
          method: "POST",
          credentials: "include",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ username: "admin", password: pw }),
        });

        const result: AttemptResult = {
          index: i + 1,
          password: pw,
          status: response.status,
          success: response.ok,
        };
        results.push(result);
        setAttemptResults([...results]);
      } catch {
        results.push({
          index: i + 1,
          password: pw,
          status: 0,
          success: false,
        });
        setAttemptResults([...results]);
      }
    }

    setBruteForceRunning(false);
  };

  const successCount = attemptResults.filter((r) => r.success).length;
  const failCount = attemptResults.filter((r) => !r.success).length;

  return (
    <div className="container">
      <div className="banner banner-danger">
        このアプリケーションは学習目的で意図的に脆弱に作られています
      </div>
      <h1>認証の不備 - 脆弱版</h1>

      {!loggedIn ? (
        <>
          <form onSubmit={handleLogin} className="login-form">
            <input
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="ユーザー名"
            />
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="パスワード"
            />
            <button type="submit" className="button button-primary">
              ログイン
            </button>
          </form>

          {error && <p className="error">{error}</p>}

          <div className="brute-force-section">
            <h2>ブルートフォース攻撃デモ</h2>
            <p>
              adminユーザーに対して、よく使われるパスワード20個を連続で試行します。
              レート制限がないため、全ての試行が即座に処理されます。
            </p>
            <button
              onClick={() => void handleBruteForce()}
              disabled={bruteForceRunning}
              className="button button-warning"
              style={{ marginTop: "12px" }}
            >
              {bruteForceRunning
                ? "実行中..."
                : "ブルートフォース攻撃を実行"}
            </button>

            {attemptResults.length > 0 && (
              <div className="brute-force-results">
                <p>
                  成功: {successCount} / 失敗: {failCount} / 合計:{" "}
                  {attemptResults.length}
                </p>
                <div className="attempt-log">
                  {attemptResults.map((r) => (
                    <p
                      key={r.index}
                      className={
                        r.success ? "attempt-success" : "attempt-fail"
                      }
                    >
                      [{r.index}] パスワード: {r.password} → ステータス:{" "}
                      {r.status} ({r.success ? "成功" : "失敗"})
                    </p>
                  ))}
                </div>
              </div>
            )}
          </div>
        </>
      ) : (
        <>
          <div className="user-info">
            <h2>管理者ページ</h2>
            {adminMessage && <p>{adminMessage}</p>}
            {userInfo && (
              <>
                <p>
                  ユーザーID: {userInfo.id}
                </p>
                <p>
                  ユーザー名: {userInfo.username}
                </p>
                <p>
                  パスワード（平文）: {userInfo.password}
                </p>
              </>
            )}
          </div>

          <div className="actions">
            <button
              onClick={() => void handleLogout()}
              className="button button-danger"
            >
              ログアウト
            </button>
          </div>
        </>
      )}

      <div className="vulnerability-hints">
        <h2>この脆弱版の問題点</h2>
        <ul>
          <li>パスワードが平文で保存されています</li>
          <li>レート制限がありません</li>
          <li>セッションIDが再生成されません</li>
        </ul>
      </div>
    </div>
  );
}

export default App;
