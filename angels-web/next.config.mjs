import withPWAInit from "@ducanh2912/next-pwa";

const withPWA = withPWAInit({
  dest: "public",
  swSrc: 'service-worker.js',
});

export default withPWA({
 
    // reactStrictMode: true,
output:"export",
    images: {
      unoptimized: true,
    }
});
