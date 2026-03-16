import { useState, useEffect, type FormEvent } from "react";

const API_URL = import.meta.env.VITE_API_URL ?? "http://localhost:8080";

interface User {
  name: string;
  email: string;
}

function App() {
  const [query, setQuery] = useState("");
  const [users, setUsers] = useState<User[]>([]);
  const [error, setError] = useState("");

  const fetchUsers = async (name?: string) => {
    try {
      setError("");
      const url = name
        ? `${API_URL}/users?name=${encodeURIComponent(name)}`
        : `${API_URL}/users`;
      const response = await fetch(url);
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }
      const data: User[] = await response.json();
      setUsers(data);
    } catch (e) {
      if (e instanceof Error) {
        setError(e.message);
      }
      setUsers([]);
    }
  };

  useEffect(() => {
    void fetchUsers();
  }, []);

  const handleSearch = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    void fetchUsers(query);
  };

  return (
    <div className="container">
      <div className="banner banner-danger">
        このアプリケーションは学習目的で意図的に脆弱に作られています
      </div>
      <h1>SQLインジェクション - 脆弱版</h1>

      <form onSubmit={handleSearch} className="search-form">
        <input
          type="text"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="ユーザー名を入力"
          className="search-input"
        />
        <button type="submit" className="search-button">
          検索
        </button>
      </form>

      {error && <p className="error">{error}</p>}

      <table className="result-table">
        <thead>
          <tr>
            <th>名前</th>
            <th>メールアドレス</th>
          </tr>
        </thead>
        <tbody>
          {users.map((user, index) => (
            <tr key={index}>
              <td>{user.name}</td>
              <td>{user.email}</td>
            </tr>
          ))}
          {users.length === 0 && (
            <tr>
              <td colSpan={2} className="no-data">
                データがありません
              </td>
            </tr>
          )}
        </tbody>
      </table>

      <div className="attack-examples">
        <h2>攻撃ペイロードの例</h2>
        <div className="example">
          <code>{`' OR '1'='1`}</code>
          <span> -- 全ユーザー取得</span>
        </div>
        <div className="example">
          <code>{`' UNION SELECT 1, sql, 3 FROM sqlite_master--`}</code>
          <span> -- テーブル構造取得</span>
        </div>
      </div>
    </div>
  );
}

export default App;
