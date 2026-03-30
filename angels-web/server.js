const express = require('express');
const next = require('next');
require('dotenv').config();

const dev = process.env.NODE_ENV !== 'production';
const app = next({ dev });
const handle = app.getRequestHandler();
require('dotenv').config();
const rateLimit = require('express-rate-limit');
const server = express();

const limiter = rateLimit({
  windowMs: 15 * 60 * 1000, // 15 minutes
  max: 100, // limit each IP to 100 requests per windowMs
  handler: function (req, res) {
    res.status(429).json({ message: "Too many requests, please try again later." });
  }
});
server.use(limiter);


app.prepare().then(() => {



  

  server.all('*', (req, res, next) => {
    console.log('Handling request:', req.path);
    return handle(req, res)
      .then(() => {
        console.log('Request handled successfully:', req.path);
      })
      .catch(err => {
        console.error('Error handling request:', req.path, err);
        next(err);
      });
  });
  

  const port = Number(process.env.PORT) || 3000;
  // 0.0.0.0 — чтобы был доступ с других устройств в LAN и из Docker (не только localhost).
  const host = process.env.HOST || '0.0.0.0';
  server.listen(port, host, (err) => {
    if (err) throw err;
    console.log(`> Ready on http://${host}:${port}`);
  });
});

