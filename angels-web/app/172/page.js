"use client"


import { Content,Grid,Column } from "@carbon/react";
import TimeToggle from "@/components/TimeToggle/TimeToggle";

import cx from 'classnames';
import Image from 'next/image'

//6 8 11 45 52 

import Pic6 from '../../public/pictures/pic6.jpg'
import Pic8 from '../../public/pictures/pic8.jpg'
import Pic11 from '../../public/pictures/pic11.jpg'
import Pic45 from '../../public/pictures/pic45.jpg'
import Pic52 from '../../public/pictures/pic52.jpg'


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
        }}> Lelahel (Лелахель) , 01:40 - 01:59 </h2>
       <div>
      <Image
        src={Pic6}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

                                                                    
<TimeToggle pageName="Исцеление Сознания 2, Галлюцинации" keyName=" 01:40 - 01:59" validationName="Lelahel" messageName="Гордость" />


<h2 style={{
          margin: '0 0 30px'
        }}> Cahetel (Кахетель) , 02:20 - 02:39 </h2>
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

                                                                    
<TimeToggle pageName="Исцеление Сознания 2, Галлюцинации" keyName=" 02:20 - 02:39" validationName="Cahetel" messageName="Гордость" />


<h2 style={{
          margin: '0 0 30px'
        }}> Lauviah (Лауиах) , 03:20 - 03:39 </h2>
       <div>
      <Image
        src={Pic11}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

                                                                    
<TimeToggle pageName="Исцеление Сознания 2, Галлюцинации" keyName=" 03:20 - 03:39" validationName="Lauviah" messageName="Гордость" />


<h2 style={{
          margin: '0 0 30px'
        }}> Sealiah (Сеалиах) , 14:40 - 14:59 </h2>
       <div>
      <Image
        src={Pic45}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

                                                                    
<TimeToggle pageName="Исцеление Сознания 2, Галлюцинации" keyName=" 14:40 - 14:59" validationName="Sealiah" messageName="Гордость" />


<h2 style={{
          margin: '0 0 30px'
        }}> Imamiah (Имамиах) , 17:00 - 17:19 </h2>
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

                                                                    
<TimeToggle pageName="Исцеление Сознания 2, Галлюцинации" keyName=" 17:00 - 17:19" validationName="Imamiah" messageName="Гордость" />


    


   
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
