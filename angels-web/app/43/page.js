"use client"


import { Content,Grid,Column } from "@carbon/react";
import TimeToggle from "@/components/TimeToggle/TimeToggle";

import cx from 'classnames';
import Image from 'next/image'


import Pic20 from '../../public/pictures/pic20.jpg'
import Pic33 from '../../public/pictures/pic33.jpg'
import Pic48 from '../../public/pictures/pic48.jpg'



import styles from '../../app/case.module.css'
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
    <div>
          <h2 style={{
          margin: '0 0 30px'
        }}>Pahaliah (Пахалиах),  06:20 - 06:39 </h2>
       <div>
      <Image
        src={Pic20}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

<TimeToggle pageName="Исцеление Тела, Мужское" keyName="06:20 - 06:39" validationName="Pahaliah" messageName="Импотенция" />



   <h2 style={{
          margin: '0 0 30px'
        }}>Yehuiah (Иехюиах), 10:40 - 10:59</h2>
       <div>
      <Image
        src={Pic33}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

<TimeToggle pageName="Исцеление Тела, Мужское" keyName="10:40 - 10:59" validationName="Yehuiah" messageName="Импотенция" />


   <h2 style={{
          margin: '0 0 30px'
        }}>Mihael (Михаёль), 15:40 - 15:59</h2>
       <div>
      <Image
        src={Pic48}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

<TimeToggle pageName="Исцеление Тела, Мужское" keyName="15:40 - 15:59" validationName="Mihael" messageName="Импотенция" />
   
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
  return(<div>
    <StoryContent/>
  </div>);
}
