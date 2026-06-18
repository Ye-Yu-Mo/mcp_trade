import { useState } from "react";
import { api } from "../api/client";

export function ConfigForm({ onConnect }: { onConnect: () => void }) {
  const [url, setUrl] = useState(localStorage.getItem("trade_server_url") || "http://185.239.224.208:8877");
  const [token, setToken] = useState(localStorage.getItem("trade_api_token") || "mcp-trade-secret-token-2024");

  const connect = () => {
    api.setConfig(url, token);
    onConnect();
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-[#0a0a0f]">
      <div className="backdrop-blur-xl bg-white/5 border border-white/10 rounded-2xl p-8 w-full max-w-md space-y-4">
        <h2 className="text-xl font-semibold text-gray-200 text-center">Trading Dashboard</h2>
        <input className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-3 text-gray-200 text-sm focus:outline-none focus:border-purple-500" placeholder="Server URL" value={url} onChange={e => setUrl(e.target.value)} />
        <input className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-3 text-gray-200 text-sm focus:outline-none focus:border-purple-500" placeholder="API Token" value={token} onChange={e => setToken(e.target.value)} type="password" />
        <button className="w-full bg-purple-600 hover:bg-purple-500 text-white rounded-lg py-3 font-medium transition-colors" onClick={connect}>Connect</button>
      </div>
    </div>
  );
}
