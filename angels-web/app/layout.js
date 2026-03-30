import './globals.css';

export const metadata = {
  title: 'Книга ангелов — заметка',
  description: 'Экспорт и уведомления',
};

export default function RootLayout({ children }) {
  return (
    <html lang="ru">
      <body>{children}</body>
    </html>
  );
}
