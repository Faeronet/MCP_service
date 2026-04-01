"use client"


import { Content,Grid,Column } from "@carbon/react";
import TimeToggle from "@/components/TimeToggle/TimeToggle";

import cx from 'classnames';
import Image from 'next/image'

// 49 64 71
import Pic49 from '../../public/pictures/pic49.jpg'
import Pic64 from '../../public/pictures/pic64.jpg'
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
        }}> Vehuel (Вехюель) , 16:00 - 16:19 </h2>
       <div>
      <Image
        src={Pic49}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

                                                                                                          
<TimeToggle pageName="Исцеление Сознания 2, Каждый сам за себя" keyName=" 16:00 - 16:19" validationName="Vehuel" messageName="Критик,человек который оказывает негативное влияние" />

   

<h2 style={{
          margin: '0 0 30px'
        }}> Mehiel (Мехиель)  , 21:00 - 21:19 </h2>
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

                                                                                                                
<TimeToggle pageName="Исцеление Сознания 2, Каждый сам за себя" keyName=" 21:00 - 21:19" validationName="Mehiel" messageName="Критик,человек который оказывает негативное влияние" />



<h2 style={{
          margin: '0 0 30px'
        }}> Haiaiel (Хаиаиель) , 23:20 - 23:39  </h2>
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
                                                                                                              
<TimeToggle pageName="Исцеление Сознания 2, Каждый сам за себя" keyName=" 23:20 - 23:39" validationName="Haiaiel" messageName="Критик,человек который оказывает негативное влияние" />

   
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
