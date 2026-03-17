import { useState, type FormEvent } from "react";

const API_URL = import.meta.env.VITE_API_URL ?? "http://localhost:8092";

interface UserInfo {
  username: string;
}

interface ApiMessage {
  message: string;
}

function App() {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [loggedInUser, setLoggedInUser] = useState<UserInfo | null>(null);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  const handleLogin = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setError("");
    setMessage("");

    try {
      const response = await fetch(`${API_URL}/login`, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username, password }),
      });

      if (!response.ok) {
        const text = await response.text();
        throw new Error(text || "ログインに失敗しました");
      }

      const data: ApiMessage = await response.json();
      setMessage(data.message);

      const meResponse = await fetch(`${API_URL}/me`, {
        credentials: "include",
      });
      if (meResponse.ok) {
        const userData: UserInfo = await meResponse.json();
        setLoggedInUser(userData);
      }

      setUsername("");
      setPassword("");
    } catch (e) {
      if (e instanceof Error) {
        setError(e.message);
      }
    }
  };

  const handleChangePassword = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    setError("");
    setMessage("");

    try {
      // カスタムオリジンヘッダーを含めてパスワード変更リクエストを送信する
      const response = await fetch(`${API_URL}/change-password`, {
        method: "POST",
        credentials: "include",
        headers: {
          "Content-Type": "application/json",
          "X-Custom-Origin": window.location.origin,
        },
        body: JSON.stringify({ new_password: newPassword }),
      });

      if (!response.ok) {
        const text = await response.text();
        throw new Error(text || "パスワード変更に失敗しました");
      }

      const data: ApiMessage = await response.json();
      setMessage(data.message);
      setNewPassword("");
    } catch (e) {
      if (e instanceof Error) {
        setError(e.message);
      }
    }
  };

  const handleLogout = async () => {
    try {
      await fetch(`${API_URL}/logout`, {
        method: "POST",
        credentials: "include",
      });
      setLoggedInUser(null);
      setMessage("ログアウトしました");
    } catch (e) {
      if (e instanceof Error) {
        setError(e.message);
      }
    }
  };

  return (
    <div className="container">
      <div className="banner banner-safe">
        このアプリケーションはCSRF対策が実装されています（SPA向け:
        カスタムオリジンヘッダー）
      </div>
      <h1>CSRF（クロスサイトリクエストフォージェリ） - 対策版（SPA向け）</h1>

      {error && <p className="error">{error}</p>}
      {message && <p className="success">{message}</p>}

      {!loggedInUser ? (
        <form onSubmit={handleLogin} className="form">
          <h2>ログイン</h2>
          <div className="form-group">
            <label htmlFor="username">ユーザー名</label>
            <input
              id="username"
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder="ユーザー名を入力"
              className="form-input"
              required
            />
          </div>
          <div className="form-group">
            <label htmlFor="password">パスワード</label>
            <input
              id="password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="パスワードを入力"
              className="form-input"
              required
            />
          </div>
          <button type="submit" className="form-button">
            ログイン
          </button>
        </form>
      ) : (
        <div>
          <div className="user-info">
            <h2>ユーザー情報</h2>
            <p>
              ログイン中: <strong>{loggedInUser.username}</strong>
            </p>
            <button onClick={handleLogout} className="logout-button">
              ログアウト
            </button>
          </div>

          <form onSubmit={handleChangePassword} className="form">
            <h2>パスワード変更</h2>
            <div className="form-group">
              <label htmlFor="newPassword">新しいパスワード</label>
              <input
                id="newPassword"
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                placeholder="新しいパスワードを入力"
                className="form-input"
                required
              />
            </div>
            <button type="submit" className="form-button">
              パスワードを変更
            </button>
          </form>
        </div>
      )}
    </div>
  );
}

export default App;
