"use client"


import { Content,Grid,Column } from "@carbon/react";
import TimeToggle from "@/components/TimeToggle/TimeToggle";

import cx from 'classnames';
import Image from 'next/image'

// 2 9 38 
import Pic2 from '../../public/pictures/pic2.jpg'
import Pic9 from '../../public/pictures/pic9.jpg'
import Pic38 from '../../public/pictures/pic38.jpg'


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
        }}>Jeliel «Иелиель», 00:20 - 00:39</h2>
       <div>
      <Image
        src={Pic2}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

                                                                  
<TimeToggle pageName="Исцеление Отношений, Любовь" keyName="00:20 - 00:39" validationName="Jeliel" messageName="Откровенность и преданность в любви" />
   


<h2 style={{
          margin: '0 0 30px'
        }}>Haziel (Хазиель), 02:40 - 02:59</h2>
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

                                                                  
<TimeToggle pageName="Исцеление Отношений, Любовь" keyName="02:40 - 02:59" validationName="Haziel" messageName="Откровенность и преданность в любви" />
   

<h2 style={{
          margin: '0 0 30px'
        }}>Haamiah (Хаамиах), 12:20 - 12:39</h2>
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

                                                                  
<TimeToggle pageName="Исцеление Отношений, Любовь" keyName="12:20 - 12:39" validationName="Haamiah" messageName="Откровенность и преданность в любви" />
   
   
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
