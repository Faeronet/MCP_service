"use client"

import Image from 'next/image';  // Importing Image from Next.js
import TimeToggle from '@/components/TimeToggle/TimeToggle';
import styles from './JustBackground.module.scss';

export default function JustBackground({ backgroundImage, title, text, timeToggleProps }) {
  return (
    <div>
      {/* <div
        className={styles.backgroundContainer}
        style={{
          backgroundImage: `url(${backgroundImage})`, // Background image passed as prop
        }}
      >  */}
        <div className={styles.textOverlay}>
          <h1>{title}</h1> {/* Title passed as prop */}
     {/* Text passed as prop */}

 	{/* <div className={styles.middleImage}>
            <Image 
              src={middleImage} 
              alt="Middle Section Image"
              width={400} 
              height={200} 
              objectFit="cover"
            />
          </div>         */}


   <Image
        src={backgroundImage}
        alt="Example Image"
        className={styles.responsiveImage}
         width={400} 
              height={200}

      />

                 


 
          
                   
          <TimeToggle 
            pageName={timeToggleProps.pageName} 
            keyName={timeToggleProps.keyName} 
            validationName={timeToggleProps.validationName} 
            messageName={timeToggleProps.messageName} 
          />
        </div>
      {/* </div> */}
    </div>
  );
}
