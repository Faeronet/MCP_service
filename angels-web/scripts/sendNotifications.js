const fs = require('fs');
const path = require('path');
const webPush = require('web-push');
const cron = require('node-cron');
require('dotenv').config();

// console.log(process.env.NEXT_PUBLIC_VAPID_PUBLIC_KEY)
webPush.setVapidDetails(
    `mailto:${process.env.WEB_PUSH_EMAIL}`,
    process.env.NEXT_PUBLIC_WEB_PUSH_PUBLIC_KEY,
    process.env.WEB_PUSH_PRIVATE_KEY
  );

const dataPath = path.join(__dirname, '../data/notifications.json');

const sendNotification = (subscription, notificationTime) => {
  const payload = JSON.stringify({
    title: 'Scheduled Notification',
    body: 'This is your scheduled notification!',
  });

  webPush.sendNotification(subscription, payload)
    .then(() => {
      console.log('Notification sent');
    })
    .catch(error => {
      console.error('Error sending notification', error);
    });
};

cron.schedule('* * * * * *', () => {
  const data = fs.readFileSync(dataPath, 'utf-8');
  const notifications = JSON.parse(data);
  const currentTime = new Date().toLocaleTimeString('en-US', { hour12: false }).slice(0, 5);
 console.log(currentTime)
  notifications.forEach((notification, index) => {
    if (notification.notificationTime === currentTime) {
      sendNotification(notification.subscription, notification.notificationTime);
      notifications.splice(index, 1); // Remove the sent notification
    }
  });

  fs.writeFileSync(dataPath, JSON.stringify(notifications, null, 2));
});
