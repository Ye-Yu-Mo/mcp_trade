export interface Config {
  tradingServerUrl: string;
  tradingApiToken: string;
}

export function loadConfig(): Config {
  const url = process.env.TRADING_SERVER_URL;
  const token = process.env.TRADING_API_TOKEN;

  if (!url) throw new Error("TRADING_SERVER_URL is required");
  if (!token) throw new Error("TRADING_API_TOKEN is required");

  return { tradingServerUrl: url.replace(/\/$/, ""), tradingApiToken: token };
}
