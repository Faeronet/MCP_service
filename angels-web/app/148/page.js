"use client"


import { Content,Grid,Column } from "@carbon/react";
import TimeToggle from "@/components/TimeToggle/TimeToggle";

import cx from 'classnames';
import Image from 'next/image'
//3 12 33 38  44 62 65 71

import Pic3 from '../../public/pictures/pic3.jpg'
import Pic12 from '../../public/pictures/pic12.jpg'
import Pic33 from '../../public/pictures/pic33.jpg'
import Pic38 from '../../public/pictures/pic38.jpg'
import Pic44 from '../../public/pictures/pic44.jpg'
import Pic62 from '../../public/pictures/pic62.jpg'
import Pic65 from '../../public/pictures/pic65.jpg'
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
        }}> Sitael «Ситаель», 00:40 - 00:59</h2>
       <div>
      <Image
        src={Pic3}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

        
<TimeToggle pageName="Исцеление Сознания 2, Авторитаризм" keyName="00:40 - 00:59" validationName="Sitael" messageName="Агрессия" />



<h2 style={{
          margin: '0 0 30px'
        }}> Hahaiah (Хахаиах), 03:40 - 03:59</h2>
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

               
<TimeToggle pageName="Исцеление Сознания 2, Авторитаризм" keyName="03:40 - 03:59" validationName="Hahaiah" messageName="Агрессия" />


<h2 style={{
          margin: '0 0 30px'
        }}> Yehuiah (Иехюиах), 10:40 - 10:59</h2>
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

                      
<TimeToggle pageName="Исцеление Сознания 2, Авторитаризм" keyName="10:40 - 10:59" validationName="Yehuiah" messageName="Агрессия" />


<h2 style={{
          margin: '0 0 30px'
        }}> Haamiah (Хаамиах), 12:20 - 12:39</h2>
       <div>
      <Image
        src={Pic38}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

                           
<TimeToggle pageName="Исцеление Сознания 2, Авторитаризм" keyName="12:20 - 12:39" validationName="Haamiah" messageName="Агрессия" />


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

                                  
<TimeToggle pageName="Исцеление Сознания 2, Авторитаризм" keyName="14:20 - 14:39" validationName="Yelahiah" messageName="Агрессия" />


<h2 style={{
          margin: '0 0 30px'
        }}> Iahhel (Иаххель), 20:20 - 20:39</h2>
       <div>
      <Image
        src={Pic62}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

                                        
<TimeToggle pageName="Исцеление Сознания 2, Авторитаризм" keyName="20:20 - 20:39" validationName="Iahhel" messageName="Агрессия" />


<h2 style={{
          margin: '0 0 30px'
        }}> Damabiah (Дамабиах), 21:20 - 21:39</h2>
       <div>
      <Image
        src={Pic65}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

                                               
<TimeToggle pageName="Исцеление Сознания 2, Авторитаризм" keyName="21:20 - 21:39" validationName="Damabiah" messageName="Агрессия" />


<h2 style={{
          margin: '0 0 30px'
        }}> Haiaiel (Хаиаиель), 23:20 - 23:39</h2>
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

                                               
<TimeToggle pageName="Исцеление Сознания 2, Авторитаризм" keyName="23:20 - 23:39" validationName="Haiaiel" messageName="Агрессия" />


   
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
