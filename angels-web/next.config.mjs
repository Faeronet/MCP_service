import withPWAInit from "@ducanh2912/next-pwa";

const withPWA = withPWAInit({
  dest: "public",
  swSrc: 'service-worker.js',
});

export default withPWA({
  // reactStrictMode: true,
  // Без output: "export": нужен обычный Next-сервер (server.js + Route Handlers вроде /api/schedule).
  images: {
    unoptimized: true,
  },
});
