import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  async rewrites() {
    return [
      {
        source: "/api/v1/:path*",
        destination: process.env.NEXT_PUBLIC_GATEWAY_HTTP
          ? `${process.env.NEXT_PUBLIC_GATEWAY_HTTP}/:path*`
          : "http://localhost:3001/api/v1/:path*", // Proxy API requests to backend
      },
    ];
  },
};

export default nextConfig;
