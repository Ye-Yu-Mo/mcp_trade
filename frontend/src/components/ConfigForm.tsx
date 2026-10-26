import { useState } from "react";
import { api } from "../api/client";

export function ConfigForm({ onConnect }: { onConnect: () => void }) {
  const [url, setUrl] = useState(
    localStorage.getItem("trade_server_url") || window.location.origin,
  );
  const [token, setToken] = useState(localStorage.getItem("trade_api_token") || "");
  const [showToken, setShowToken] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const connect = async () => {
    setError("");
    if (!url.trim() || !token.trim()) {
      setError("请填写服务器地址与 API Token");
      return;
    }

    setLoading(true);
    api.setConfig(url.trim().replace(/\/$/, ""), token.trim(), false);
    const balances = await api.balance();
    setLoading(false);

    if (balances === null) {
      setError("连接失败：请检查地址、Token，以及 trading-server 是否在运行");
      return;
    }
    // Persist only after the credentials have been verified.
    api.setConfig(url.trim().replace(/\/$/, ""), token.trim(), true);
    onConnect();
  };

  return (
    <div className="relative flex min-h-screen items-center justify-center overflow-hidden p-4">
      <div className="pointer-events-none absolute left-1/2 top-1/2 h-[620px] w-[620px] -translate-x-1/2 -translate-y-1/2 rounded-full bg-[var(--accent)]/10 blur-[120px]" />

      <div className="panel relative z-10 w-full max-w-[420px] p-7">
        <div className="mb-7 flex items-center gap-3">
          <div className="flex h-11 w-11 items-center justify-center rounded-xl border border-[rgba(57,135,229,0.35)] bg-[var(--accent-soft)] text-[#9cc4ff]">
            <svg
              viewBox="0 0 24 24"
              className="h-5 w-5"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <polyline points="22 12 18 12 15 21 9 3 6 12 2 12" />
            </svg>
          </div>
          <div>
            <h1 className="text-[17px] font-semibold tracking-tight text-[var(--ink)]">
              MCP Trade
            </h1>
            <p className="mt-0.5 text-[11px] text-[var(--ink-3)]">AI 主观交易终端</p>
          </div>
        </div>

        <div className="space-y-4">
          <div>
            <label className="mb-1.5 block text-[11px] font-medium text-[var(--ink-2)]">
              服务器地址
            </label>
            <input
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              autoComplete="off"
              spellCheck={false}
              placeholder="http://127.0.0.1:8877"
              className="w-full rounded-lg border border-[var(--line)] bg-[rgba(2,6,14,0.55)] px-3.5 py-2.5 text-[13px] text-[var(--ink)] placeholder:text-[var(--ink-3)] transition-colors focus:border-[rgba(57,135,229,0.6)] focus:outline-none"
            />
          </div>

          <div>
            <label className="mb-1.5 block text-[11px] font-medium text-[var(--ink-2)]">
              API Token
            </label>
            <div className="relative">
              <input
                type={showToken ? "text" : "password"}
                value={token}
                onChange={(e) => setToken(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") void connect();
                }}
                autoComplete="off"
                spellCheck={false}
                placeholder="trading-server/.env 中的 API_TOKEN"
                className="w-full rounded-lg border border-[var(--line)] bg-[rgba(2,6,14,0.55)] px-3.5 py-2.5 pr-16 text-[13px] text-[var(--ink)] placeholder:text-[var(--ink-3)] transition-colors focus:border-[rgba(57,135,229,0.6)] focus:outline-none"
              />
              <button
                type="button"
                onClick={() => setShowToken((v) => !v)}
                className="absolute right-2 top-1/2 -translate-y-1/2 rounded px-2 py-1 text-[10px] text-[var(--ink-3)] transition-colors hover:text-[var(--ink-2)]"
              >
                {showToken ? "隐藏" : "显示"}
              </button>
            </div>
          </div>

          {error && (
            <div className="rounded-lg border border-[rgba(208,59,59,0.3)] bg-[rgba(208,59,59,0.08)] px-3 py-2 text-[11px] text-[#ff9b9b]">
              {error}
            </div>
          )}

          <button
            type="button"
            onClick={() => void connect()}
            disabled={loading}
            className="flex w-full items-center justify-center gap-2 rounded-lg bg-[var(--accent)] py-2.5 text-[13px] font-medium text-white transition-opacity hover:opacity-90 disabled:opacity-60"
          >
            {loading ? (
              <>
                <svg
                  className="h-4 w-4 animate-spin"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                >
                  <path d="M21 12a9 9 0 1 1-6.219-8.56" />
                </svg>
                连接中…
              </>
            ) : (
              <>
                <svg
                  className="h-4 w-4"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2" />
                </svg>
                建立连接
              </>
            )}
          </button>
        </div>

        <p className="mt-5 text-center text-[10px] leading-relaxed text-[var(--ink-3)]">
          Token 仅保存在本机浏览器 localStorage，不会上传。
        </p>
      </div>
    </div>
  );
}
