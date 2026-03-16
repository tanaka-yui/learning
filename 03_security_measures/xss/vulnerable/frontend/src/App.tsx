import { useState, useEffect, type FormEvent } from "react";

const API_URL = import.meta.env.VITE_API_URL ?? "http://localhost:8082";

interface Post {
  id: number;
  content: string;
  createdAt: string;
}

function App() {
  const [content, setContent] = useState("");
  const [posts, setPosts] = useState<Post[]>([]);
  const [error, setError] = useState("");

  const fetchPosts = async () => {
    try {
      setError("");
      const response = await fetch(`${API_URL}/posts`);
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }
      const data: Post[] = await response.json();
      setPosts(data);
    } catch (e) {
      if (e instanceof Error) {
        setError(e.message);
      }
      setPosts([]);
    }
  };

  useEffect(() => {
    void fetchPosts();
  }, []);

  const handleSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!content.trim()) return;

    try {
      setError("");
      const response = await fetch(`${API_URL}/posts`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ content }),
      });
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }
      setContent("");
      await fetchPosts();
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
      <h1>XSS(クロスサイトスクリプティング) - 脆弱版</h1>

      <form onSubmit={(e) => void handleSubmit(e)} className="post-form">
        <textarea
          value={content}
          onChange={(e) => setContent(e.target.value)}
          placeholder="投稿内容を入力"
          className="post-input"
          rows={3}
        />
        <button type="submit" className="post-button">
          投稿
        </button>
      </form>

      {error && <p className="error">{error}</p>}

      <div className="posts">
        <h2>投稿一覧</h2>
        {posts.length === 0 && <p className="no-data">投稿がありません</p>}
        {posts.map((post) => (
          <div key={post.id} className="post-card">
            <div
              className="post-content"
              dangerouslySetInnerHTML={{ __html: post.content }}
            />
            <div className="post-date">{post.createdAt}</div>
          </div>
        ))}
      </div>

      <div className="attack-examples">
        <h2>攻撃ペイロードの例</h2>
        <div className="example">
          <code>{`<script>alert('XSS')</script>`}</code>
          <span> -- スクリプト実行</span>
        </div>
        <div className="example">
          <code>{`<img onerror="alert('XSS')" src="x">`}</code>
          <span> -- イベントハンドラ</span>
        </div>
        <div className="example">
          <code>{`<a href="javascript:alert('XSS')">Click</a>`}</code>
          <span> -- JavaScript URL</span>
        </div>
      </div>
    </div>
  );
}

export default App;
