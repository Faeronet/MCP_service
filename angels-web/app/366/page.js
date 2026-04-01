"use client"


import { Content,Grid,Column } from "@carbon/react";
import TimeToggle from "@/components/TimeToggle/TimeToggle";

import cx from 'classnames';
import Image from 'next/image'


import Pic16 from '../../public/pictures/pic16.jpg'
import Pic39 from '../../public/pictures/pic39.jpg'
import Pic60 from '../../public/pictures/pic60.jpg'

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
        }}>Hekamiah (Хакамиах), 05:00 - 05:19</h2>
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

                                  
<TimeToggle pageName="Исцеление Сознания 2, Вечный сон,спячка" keyName=" 05:00 - 05:19" validationName="Hekamiah" messageName="Восстание" />



<h2 style={{
          margin: '0 0 30px'
        }}>Rehael (Рехаёль), 12:40 - 12:59</h2>
       <div>
      <Image
        src={Pic39}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

                                  
<TimeToggle pageName="Исцеление Сознания 2, Вечный сон,спячка" keyName=" 12:40 - 12:59" validationName="Rehael" messageName="Восстание" />



<h2 style={{
          margin: '0 0 30px'
        }}>Mitzrael (Мизраель), 19:40 - 19:59</h2>
       <div>
      <Image
        src={Pic60}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

                                  
<TimeToggle pageName="Исцеление Сознания 2, Вечный сон,спячка" keyName=" 19:40 - 19:59" validationName="Mitzrael" messageName="Восстание" />




   
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
