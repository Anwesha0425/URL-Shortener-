"use client";

import { useState } from "react";

interface ShortenResult {
  short_code: string;
  short_url: string;
  original_url: string;
}

export default function UrlShortenerForm() {
  const [url, setUrl] = useState("");
  const [alias, setAlias] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [result, setResult] = useState<ShortenResult | null>(null);
  const [error, setError] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError("");
    setResult(null);

    try {
      const res = await fetch("http://localhost:8001/api/v1/urls", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          original_url: url,
          custom_alias: alias || undefined,
        }),
      });

      const data = await res.json();

      if (!res.ok) {
        throw new Error(data.error || "Failed to shorten URL");
      }

      setResult(data);
      setUrl("");
      setAlias("");
    } catch (err: any) {
      setError(err.message);
    } finally {
      setIsLoading(false);
    }
  };

  const copyToClipboard = () => {
    if (result) {
      navigator.clipboard.writeText(result.short_url);
    }
  };

  return (
    <div className="w-full max-w-xl mx-auto mt-12 animate-float">
      <div className="glass-panel p-8">
        <form onSubmit={handleSubmit} className="flex flex-col gap-5">
          <div>
            <label className="block text-sm font-medium text-white/70 mb-2">
              Destination URL
            </label>
            <input
              type="url"
              required
              placeholder="https://example.com/very-long-link"
              className="glass-input w-full"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-white/70 mb-2">
              Custom Alias <span className="text-white/40 text-xs">(Optional)</span>
            </label>
            <div className="flex items-center gap-3">
              <span className="text-white/50 bg-black/20 px-3 py-3 rounded-xl border border-white/5 select-none hidden sm:block">
                sho.rt/
              </span>
              <input
                type="text"
                placeholder="my-campaign"
                className="glass-input flex-1"
                value={alias}
                onChange={(e) => setAlias(e.target.value)}
                pattern="[a-zA-Z0-9-]+"
                title="Only letters, numbers, and hyphens"
              />
            </div>
          </div>

          {error && (
            <div className="bg-red-500/10 border border-red-500/20 text-red-400 px-4 py-3 rounded-xl text-sm">
              {error}
            </div>
          )}

          <button
            type="submit"
            disabled={isLoading || !url}
            className="glowing-btn mt-2 disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
          >
            {isLoading ? (
              <>
                <svg className="animate-spin h-5 w-5 text-white" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                Shortening...
              </>
            ) : (
              "Shorten Link"
            )}
          </button>
        </form>

        {/* Result Card */}
        {result && (
          <div className="mt-8 pt-8 border-t border-white/10 animate-fade-in">
            <h3 className="text-sm font-medium text-white/70 mb-4">Your Link is Ready</h3>
            <div className="flex items-center justify-between bg-black/40 border border-purple-500/30 p-4 rounded-xl">
              <a 
                href={result.short_url} 
                target="_blank" 
                rel="noreferrer"
                className="text-purple-400 font-medium hover:text-purple-300 truncate mr-4 transition-colors"
              >
                {result.short_url}
              </a>
              <button 
                onClick={copyToClipboard}
                className="text-white/60 hover:text-white bg-white/5 hover:bg-white/10 p-2 rounded-lg transition-colors"
                title="Copy to clipboard"
              >
                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"></path>
                </svg>
              </button>
            </div>
            
            <div className="mt-4 flex justify-end">
              <a 
                href="http://localhost:8003/dashboard" 
                target="_blank" 
                rel="noreferrer"
                className="text-sm text-blue-400 hover:text-blue-300 flex items-center gap-1 transition-colors"
              >
                View Live Analytics 
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z"></path>
                </svg>
              </a>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
