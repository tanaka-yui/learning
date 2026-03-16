import { useState, useEffect, type FormEvent } from "react";

const API_URL = import.meta.env.VITE_API_URL ?? "http://localhost:8088";

function App() {
  const [files, setFiles] = useState<string[]>([]);
  const [filePath, setFilePath] = useState("");
  const [fileContent, setFileContent] = useState("");
  const [error, setError] = useState("");
  const [statusCode, setStatusCode] = useState<number | null>(null);

  const fetchFileList = async () => {
    try {
      const response = await fetch(`${API_URL}/files`);
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }
      const data: string[] = await response.json();
      setFiles(data);
    } catch (e) {
      if (e instanceof Error) {
        setError(e.message);
      }
    }
  };

  const downloadFile = async (filename: string) => {
    try {
      setError("");
      setFileContent("");
      setStatusCode(null);
      const response = await fetch(
        `${API_URL}/download?file=${encodeURIComponent(filename)}`
      );
      setStatusCode(response.status);
      if (!response.ok) {
        const text = await response.text();
        throw new Error(`HTTP ${response.status}: ${text}`);
      }
      const text = await response.text();
      setFileContent(text);
    } catch (e) {
      if (e instanceof Error) {
        setError(e.message);
      }
      setFileContent("");
    }
  };

  useEffect(() => {
    void fetchFileList();
  }, []);

  const handleSubmit = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    void downloadFile(filePath);
  };

  return (
    <div className="container">
      <div className="banner banner-danger">
        このアプリケーションは学習目的で意図的に脆弱に作られています
      </div>
      <h1>パストラバーサル - 脆弱版</h1>

      <div className="section">
        <h2>ファイル一覧</h2>
        <div className="file-list">
          {files.map((file) => (
            <button
              key={file}
              className="file-button"
              onClick={() => void downloadFile(file)}
            >
              {file}
            </button>
          ))}
          {files.length === 0 && (
            <p className="no-data">ファイルがありません</p>
          )}
        </div>
      </div>

      <div className="section">
        <h2>ファイルパスを手動入力</h2>
        <form onSubmit={handleSubmit} className="search-form">
          <input
            type="text"
            value={filePath}
            onChange={(e) => setFilePath(e.target.value)}
            placeholder="ファイルパスを入力（例: readme.txt）"
            className="search-input"
          />
          <button type="submit" className="search-button">
            ダウンロード
          </button>
        </form>
      </div>

      {statusCode !== null && (
        <p className={statusCode >= 400 ? "error" : "success"}>
          ステータスコード: {statusCode}
        </p>
      )}
      {error && <p className="error">{error}</p>}

      {fileContent && (
        <div className="content-area">
          <h2>ファイル内容</h2>
          <pre>
            <code>{fileContent}</code>
          </pre>
        </div>
      )}

      <div className="attack-examples">
        <h2>攻撃ペイロードの例</h2>
        <div className="example">
          <code>../main.go</code>
          <span> -- ソースコード取得</span>
        </div>
        <div className="example">
          <code>../../etc/passwd</code>
          <span> -- システムファイル取得</span>
        </div>
        <div className="example">
          <code>secret/credentials.txt</code>
          <span> -- 秘密ファイル取得</span>
        </div>
      </div>
    </div>
  );
}

export default App;
