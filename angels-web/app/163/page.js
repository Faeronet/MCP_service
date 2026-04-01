"use client"


import { Content,Grid,Column } from "@carbon/react";
import TimeToggle from "@/components/TimeToggle/TimeToggle";

import cx from 'classnames';
import Image from 'next/image'

// 9 , 16 , 34, 43 , 44,  71
import Pic9 from '../../public/pictures/pic9.jpg'
import Pic16 from '../../public/pictures/pic16.jpg'
import Pic34 from '../../public/pictures/pic34.jpg'
import Pic43 from '../../public/pictures/pic43.jpg'
import Pic44 from '../../public/pictures/pic44.jpg'
import Pic71 from '../../public/pictures/pic71.jpg'

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
        }}> Haziel (Хазиель), 02:40 - 02:59</h2>
       <div>
      <Image
        src={Pic9}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

                                         
<TimeToggle pageName="Исцеление Сознания 2, Вечный сон,спячка" keyName=" 02:40 - 02:59" validationName="Haziel" messageName="Война, постоянные конфликты" />


<h2 style={{
          margin: '0 0 30px'
        }}> Hekamiah (Хакамиах), 05:00 - 05:19</h2>
       <div>
      <Image
        src={Pic16}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

                                               
<TimeToggle pageName="Исцеление Сознания 2, Вечный сон,спячка" keyName=" 05:00 - 05:19" validationName="Hekamiah" messageName="Война, постоянные конфликты" />





<h2 style={{
          margin: '0 0 30px'
        }}> Lehahiah (Лехахиах), 11:00 - 11:19</h2>
       <div>
      <Image
        src={Pic34}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

                                                   
<TimeToggle pageName="Исцеление Сознания 2, Вечный сон,спячка" keyName=" 11:00 - 11:19" validationName="Lehahiah" messageName="Война, постоянные конфликты" />


<h2 style={{
          margin: '0 0 30px'
        }}> Veuliah (Вевалиах), 14:00 - 14:19</h2>
       <div>
      <Image
        src={Pic43}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>
                                                   
<TimeToggle pageName="Исцеление Сознания 2, Вечный сон,спячка" keyName=" 14:00 - 14:19" validationName="Veuliah" messageName="Война, постоянные конфликты" />


<h2 style={{
          margin: '0 0 30px'
        }}> Yelahiah (Иелахиах), 14:20 - 14:39</h2>
       <div>
      <Image
        src={Pic44}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

                                                   
<TimeToggle pageName="Исцеление Сознания 2, Вечный сон,спячка" keyName=" 14:20 - 14:39" validationName="Yelahiah" messageName="Война, постоянные конфликты" />



<h2 style={{
          margin: '0 0 30px'
        }}> Haiaiel (Хаиаиель) , 23:20 - 23:39</h2>
       <div>
      <Image
        src={Pic71}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>
    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

                                                   
<TimeToggle pageName="Исцеление Сознания 2, Вечный сон,спячка" keyName=" 23:20 - 23:39" validationName="Haiaiel" messageName="Война, постоянные конфликты" />

    
   
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
