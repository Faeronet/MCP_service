"use client"

import TimeToggle from "@/components/TimeToggle/TimeToggle";
import { Content,Grid,Column } from "@carbon/react";
import cx from 'classnames';
import Image from 'next/image'


import Pic8 from '../../public/pictures/pic8.jpg'
import Pic52 from '../../public/pictures/pic52.jpg'


import styles from '../../app/case.module.css'

import styles1 from '@/app/5/repo5.module.scss';

const StoryContent = ({
  useResponsiveOffset = true
}) => {
  const classNameFirstColumn = cx({
    'cds--col-lg-13': true,
    'cds--offset-lg-3': useResponsiveOffset
  });
  const content = <Grid>
     <Column sm={{
    span: 4,
    offset: 0
  }} md={{
    span: 8,
    offset: 0
  }} lg={{
    span: 12,
    offset: 4
  }} >
    {/* <div className={styles.square}> */}
    <div>
    <div/>
        
          <h2  className={styles.text}  >Cahetel (Кахетель),  02:20 - 02:39 </h2>


       <div>
      <Image
        src={Pic8}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

<h2 style={{
          margin: '0 0 30px'
        }}></h2>


    <TimeToggle pageName="Исцеление Сознания, Неверие" keyName="02:20 - 02:39" validationName="Cahetel" messageName="Богохульство" />

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>
        
 <h2 style={{
          margin: '0 0 30px'
        }}> Imamiah (Имамиах), 17:00 - 17:19 </h2>
       <div>
      <Image
        src={Pic52}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

<h2 style={{
          margin: '0 0 30px'
        }}></h2>

    <TimeToggle pageName="Исцеление Сознания, Неверие" keyName="17:00 - 17:19" validationName="Imamiah" messageName="Богохульство" />
          <h2 style={{
          margin: '0 0 30px'
        }}></h2>

      </div>
      </Column>
      </Grid>;
  const style = {
    height: '100%'
  };
  if (useResponsiveOffset) {
    style.margin = '0';
    style.width = '100%';
  }

 return <div>
	{content}
	</div>;



};

export default function RepoPage() {
  return(<div className={styles1.backgroundContainer6}>
    <StoryContent/>
  </div>);
}