import { useEffect, useState } from 'react';

const HomePage = () => {
  const [data, setData] = useState(null);
  const [name, setName] = useState('');
  const [message, setMessage] = useState('');

  useEffect(() => {
    // Check if the key exists in local storage
    const savedData = localStorage.getItem('userData');
    if (savedData) {
      setData(JSON.parse(savedData));
      setMessage('Saved data found in Local Storage');
    }
  }, []);

  const saveData = () => {
    const currentTime = new Date().toLocaleString();
    const userData = { name, time: currentTime };
    localStorage.setItem('userData', JSON.stringify(userData));
    setData(userData);
    setMessage(`Saved data: ${JSON.stringify(userData)}`);

    // Show notification
    if (Notification.permission === 'granted') {
      new Notification('Data Saved', {
        body: `Name: ${name}, Time: ${currentTime}`,
      });
    }
  };

  const requestNotificationPermission = () => {
    if (Notification.permission !== 'denied' || Notification.permission === 'default') {
      Notification.requestPermission().then((permission) => {
        if (permission === 'granted') {
          new Notification('Notification enabled');
        }
      });
    }
  };

  useEffect(() => {
    requestNotificationPermission();
  }, []);

  return (
    <div>
      <h1>Local Storage JSON Example</h1>
      {data ? (
        <p>{message}</p>
      ) : (
        <>
          <p>No data saved in Local Storage.</p>
          <input
            type="text"
            placeholder="Enter your name"
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
          <button onClick={saveData}>Save Name and Time</button>
        </>
      )}
    </div>
  );
};

export default HomePage;
