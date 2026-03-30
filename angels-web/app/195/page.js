"use client"


import { Content,Grid,Column } from "@carbon/react";
import TimeToggle from "@/components/TimeToggle/TimeToggle";

import cx from 'classnames';
import Image from 'next/image'

// 12 17 25 51 64 

import Pic12 from '../../public/pictures/pic12.jpg'
import Pic17 from '../../public/pictures/pic17.jpg'
import Pic25 from '../../public/pictures/pic25.jpg'
import Pic51 from '../../public/pictures/pic51.jpg'
import Pic64 from '../../public/pictures/pic64.jpg'


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

        
                                                                                                      
<TimeToggle pageName="Исцеление Сознания 2, Излишества" keyName="  03:40 - 03:59" validationName="Hahaiah" messageName="Иллюзии" />


<h2 style={{
          margin: '0 0 30px'
        }}> Leviah (Левиах) , 05:20 - 05:39 </h2>
       <div>
      <Image
        src={Pic17}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

        
                                                                                                      
<TimeToggle pageName="Исцеление Сознания 2, Зависть" keyName="  05:20 - 05:39" validationName="Leviah" messageName="Иллюзии" />




    <h2 style={{
          margin: '0 0 30px'
        }}></h2>
         <h2 style={{
          margin: '0 0 30px'
        }}> Nith-Haiah (Нитхаиах) , 08:00 - 08:19 </h2>
       <div>
      <Image
        src={Pic25}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

        
                                                                                                      
<TimeToggle pageName="Исцеление Сознания 2, Зависть" keyName="  08:00 - 08:19" validationName="Nith-Haiah" messageName="Иллюзии" />



    <h2 style={{
          margin: '0 0 30px'
        }}></h2>
         <h2 style={{
          margin: '0 0 30px'
        }}> Hahasiah (Хахасиах), 16:40 - 16:59 </h2>
       <div>
      <Image
        src={Pic51}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

        
                                                                                                      
<TimeToggle pageName="Исцеление Сознания 2, Зависть" keyName=" 16:40 - 16:59" validationName="Hahasiah" messageName="Иллюзии" />



    <h2 style={{
          margin: '0 0 30px'
        }}></h2>
         <h2 style={{
          margin: '0 0 30px'
        }}> Mehiel (Мехиель) , 21:00 - 21:19 </h2>
       <div>
      <Image
        src={Pic64}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

        
                                                                                                      
<TimeToggle pageName="Исцеление Сознания 2, Зависть" keyName=" 21:00 - 21:19" validationName="Mehiel" messageName="Иллюзии" />



   



   
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
