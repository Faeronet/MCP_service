import './globals.scss';
import { Providers } from './providers';

export const metadata = {
  title: 'Путь Ангелов',
  description: 'Путь Ангелов',
};

export default function RootLayout({ children }) {
  return (
    <html lang="en">
      <head>
        <link rel="manifest" href="/manifest.json" />
        <link rel="apple-touch-icon" href="/icon.png" />
        <meta name="theme-color" content="#fff" />
      </head>
      <body>
        <div className="backgroundContainer">
            
          <Providers>{children}</Providers>
        </div>
      </body>
    </html>
  );
}
