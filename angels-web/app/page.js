'use client'
import { Column, Grid } from "@carbon/react";
import Image from 'next/image';
import { useEffect } from "react";
import Pic2 from '../public/pic2.jpg';



export default function Home() {


  useEffect(() => {
    if ('Notification' in window) {
      if (Notification.permission === 'default') {
        Notification.requestPermission()
          .then((permission) => {
            if (permission === 'granted') {
              console.log('Notification permission granted.');
            } else if (permission === 'denied') {
              console.log('Notification permission denied.');
            }
          })
          .catch((error) => {
            console.error('Error requesting notification permission:', error);
          });
      }
    } else {
      console.log('This browser does not support notifications.');
    }
  }, []);

  return (
    <Grid>
     <Column sm={{
    span: 4,
    offset: 0
  }} md={{
    span: 8,
    offset: 0
  }} lg={{
    span: 13,
    offset: 3
  }} >
<div>
<Image
      alt="Mountains"
      src={Pic2}
      placeholder="blur"
      quality={100}
      width={1000}
      sizes="100vw"
    
    />
    </div>
    <div>
      <p>test app testing</p>
    </div>
 
  </Column>
  </Grid>

  );
}
