import { useState } from "react";

const API_BASE = import.meta.env.VITE_API_BASE ?? "http://localhost:9100";

export function App() {
  const [result, setResult] = useState<string>("");

  async function checkout() {
    setResult("...");
    try {
      const res = await fetch(`${API_BASE}/api/checkout`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ item: "book", qty: 1 }),
      });
      setResult(`${res.status} ${await res.text()}`);
    } catch (e) {
      setResult(`error: ${String(e)}`);
    }
  }

  return (
    <main style={{ fontFamily: "sans-serif", padding: 32 }}>
      <h1>08 Observability デモ</h1>
      <button onClick={checkout}>Checkout を実行</button>
      <pre>{result}</pre>
    </main>
  );
}
