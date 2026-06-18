import { useState } from "react";
import { api } from "../api/client";

export function ConfigForm({ onConnect }: { onConnect: () => void }) {
  const [url, setUrl] = useState(localStorage.getItem("trade_server_url") || "http://185.239.224.208:8877");
  const [token, setToken] = useState(localStorage.getItem("trade_api_token") || "mcp-trade-secret-token-2024");
  const [loading, setLoading] = useState(false);

  const connect = () => {
    setLoading(true);
    api.setConfig(url, token);
    setTimeout(() => { setLoading(false); onConnect(); }, 600);
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-slate-950 p-4 relative overflow-hidden">
      <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[600px] h-[600px] bg-indigo-600/20 rounded-full blur-[120px] pointer-events-none" />
      <div className="bg-slate-900/80 backdrop-blur-2xl border border-slate-700 rounded-2xl p-8 w-full max-w-sm relative z-10 shadow-2xl">
        <div className="flex justify-center mb-6">
          <div className="w-12 h-12 bg-indigo-500/20 text-indigo-400 rounded-xl flex items-center justify-center border border-indigo-500/30">
            <svg viewBox="0 0 24 24" className="w-6 h-6" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>
          </div>
        </div>
        <h2 className="text-xl font-bold text-slate-100 text-center mb-2">量化交易控制台</h2>
        <p className="text-slate-400 text-sm text-center mb-8">MCP 交易引擎终端</p>
        <div className="space-y-4">
          <div>
            <label className="text-xs font-medium text-slate-400 mb-1 block">服务器地址</label>
            <input value={url} onChange={e => setUrl(e.target.value)} className="w-full bg-slate-950/50 border border-slate-700 rounded-lg px-4 py-2.5 text-slate-200 text-sm focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 transition-all" />
          </div>
          <div>
            <label className="text-xs font-medium text-slate-400 mb-1 block">API 密钥</label>
            <input type="password" value={token} onChange={e => setToken(e.target.value)} className="w-full bg-slate-950/50 border border-slate-700 rounded-lg px-4 py-2.5 text-slate-200 text-sm focus:outline-none focus:border-indigo-500 focus:ring-1 focus:ring-indigo-500 transition-all" />
          </div>
          <button onClick={connect} disabled={loading} className="w-full bg-indigo-600 hover:bg-indigo-500 text-white rounded-lg py-2.5 font-medium transition-colors flex items-center justify-center gap-2 disabled:opacity-70">
            {loading ? (
              <><svg className="animate-spin w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>连接中...</>
            ) : (
              <><svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/></svg>建立连接</>
            )}
          </button>
        </div>
      </div>
    </div>
  );
}
