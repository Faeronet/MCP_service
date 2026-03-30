'use client';

import { useEffect, useState } from 'react';

export default function Home() {
  const [greeting, setGreeting] = useState('');

  const fetchGreeting = async () => {
    const response = await fetch('/api/hello');

    if (!response.ok) {
      setGreeting('Failed to fetch greeting');
      return;
    }

    const data = await response.json();
    setGreeting(data.greeting);
  };

  useEffect(() => {
    fetchGreeting();
  }, []);

  return (
    <div>
      <h1>Welcome to Next.js</h1>
      <p>{greeting}</p>
    </div>
  );
}
