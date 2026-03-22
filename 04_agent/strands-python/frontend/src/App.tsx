import { useState, useRef, useEffect } from "react";
import { v4 as uuidv4 } from "uuid";

const BACKEND_URL = (import.meta.env.VITE_BACKEND_URL as string) || "http://localhost:4003";
const THEME_COLOR = (import.meta.env.VITE_THEME_COLOR as string) || "#6366f1";
const FRAMEWORK_NAME = (import.meta.env.VITE_FRAMEWORK_NAME as string) || "Agent";
const SESSION_ID = uuidv4();

type Message = { role: "user" | "assistant"; content: string };

export default function App() {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  const send = async () => {
    if (!input.trim() || loading) return;
    const userMsg = input.trim();
    setInput("");
    setMessages((prev) => [...prev, { role: "user", content: userMsg }]);
    setLoading(true);
    try {
      const res = await fetch(`${BACKEND_URL}/chat`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ message: userMsg, sessionId: SESSION_ID }),
      });
      const data = await res.json();
      setMessages((prev) => [...prev, { role: "assistant", content: data.response }]);
    } catch {
      setMessages((prev) => [...prev, { role: "assistant", content: "エラーが発生しました。" }]);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ height: "100vh", display: "flex", flexDirection: "column", fontFamily: "sans-serif" }}>
      <header style={{ background: THEME_COLOR, color: "#fff", padding: "12px 16px" }}>
        <h1 style={{ fontSize: "18px" }}>{FRAMEWORK_NAME} Chat</h1>
        <p style={{ fontSize: "11px", opacity: 0.7, marginTop: "2px" }}>Session: {SESSION_ID}</p>
      </header>
      <div style={{ flex: 1, overflow: "auto", padding: "16px", display: "flex", flexDirection: "column", gap: "8px" }}>
        {messages.map((m, i) => (
          <div key={i} style={{ alignSelf: m.role === "user" ? "flex-end" : "flex-start", maxWidth: "70%" }}>
            <div
              style={{
                background: m.role === "user" ? THEME_COLOR : "#f3f4f6",
                color: m.role === "user" ? "#fff" : "#111",
                padding: "8px 12px",
                borderRadius: "12px",
                whiteSpace: "pre-wrap",
                fontSize: "14px",
              }}
            >
              {m.content}
            </div>
          </div>
        ))}
        {loading && <div style={{ alignSelf: "flex-start", color: "#9ca3af", fontSize: "14px" }}>...</div>}
        <div ref={bottomRef} />
      </div>
      <div style={{ padding: "12px 16px", borderTop: "1px solid #e5e7eb", display: "flex", gap: "8px" }}>
        <input
          style={{ flex: 1, padding: "8px 12px", borderRadius: "8px", border: "1px solid #d1d5db", fontSize: "14px" }}
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => e.key === "Enter" && !e.shiftKey && send()}
          placeholder="メッセージを入力..."
          disabled={loading}
        />
        <button
          onClick={send}
          disabled={loading || !input.trim()}
          style={{
            padding: "8px 16px",
            background: THEME_COLOR,
            color: "#fff",
            border: "none",
            borderRadius: "8px",
            cursor: loading || !input.trim() ? "not-allowed" : "pointer",
            opacity: loading || !input.trim() ? 0.5 : 1,
            fontSize: "14px",
          }}
        >
          送信
        </button>
      </div>
    </div>
  );
}
