import UrlShortenerForm from "@/components/UrlShortenerForm";

export default function Home() {
  return (
    <main className="min-h-screen flex flex-col items-center justify-center p-6 relative overflow-hidden">
      
      {/* Background Decorative Elements */}
      <div className="absolute top-1/4 left-1/4 w-96 h-96 bg-purple-600/20 rounded-full blur-[100px] -z-10 mix-blend-screen animate-pulse-slow"></div>
      <div className="absolute bottom-1/4 right-1/4 w-[30rem] h-[30rem] bg-blue-600/20 rounded-full blur-[120px] -z-10 mix-blend-screen animate-pulse-slow" style={{ animationDelay: '2s' }}></div>

      <div className="text-center z-10 max-w-2xl mx-auto mb-4">
        <div className="inline-flex items-center gap-2 px-4 py-2 rounded-full bg-white/5 border border-white/10 text-sm text-purple-300 font-medium mb-8 select-none">
          <span className="relative flex h-2 w-2">
            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-purple-400 opacity-75"></span>
            <span className="relative inline-flex rounded-full h-2 w-2 bg-purple-500"></span>
          </span>
          Next-Gen Infrastructure
        </div>
        
        <h1 className="text-5xl md:text-7xl font-bold tracking-tight mb-6">
          Shorten. <span className="text-gradient">Track.</span> Scale.
        </h1>
        
        <p className="text-lg text-white/60 leading-relaxed max-w-xl mx-auto">
          Built on a high-performance distributed microservice architecture. 
          Experience sub-millisecond redirects and real-time Kafka-driven analytics.
        </p>
      </div>

      <UrlShortenerForm />

      <footer className="mt-24 text-center text-white/30 text-sm">
        Architecture: Golang • Python • Node.js • Kafka • Redis • ClickHouse
      </footer>
    </main>
  );
}
