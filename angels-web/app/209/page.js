"use client"


import { Content,Grid,Column } from "@carbon/react";
import TimeToggle from "@/components/TimeToggle/TimeToggle";

import cx from 'classnames';
import Image from 'next/image'

//12 13 55
import Pic12 from '../../public/pictures/pic12.jpg'
import Pic13 from '../../public/pictures/pic13.jpg'
import Pic55 from '../../public/pictures/pic55.jpg'



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
        }}> Hahaiah (Хахаиах) , 03:40 - 03:59 </h2>
       <div>
      <Image
        src={Pic12}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

                                                                                                                   
<TimeToggle pageName="Исцеление Сознания 2, Легкомыслие" keyName=" 03:40 - 03:59" validationName="Hahaiah" messageName="Ложь" />



<h2 style={{
          margin: '0 0 30px'
        }}> Iezalel (Иезелель) , 04:00 - 04:19 </h2>
       <div>
      <Image
        src={Pic13}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

                                                                                                                   
<TimeToggle pageName="Исцеление Сознания 2, Легкомыслие" keyName=" 04:00 - 04:19" validationName="Iezalel" messageName="Ложь" />




<h2 style={{
          margin: '0 0 30px'
        }}> Mebahiah (Мебаиах) , 18:00 - 18:19 </h2>
       <div>
      <Image
        src={Pic55}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

                                                                                                                   
<TimeToggle pageName="Исцеление Сознания 2, Легкомыслие" keyName=" 18:00 - 18:19" validationName="Mebahiah" messageName="Ложь" />




   
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
