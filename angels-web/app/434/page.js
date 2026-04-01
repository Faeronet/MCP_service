"use client"


import { Content,Grid,Column } from "@carbon/react";
import TimeToggle from "@/components/TimeToggle/TimeToggle";

import cx from 'classnames';
import Image from 'next/image'

// 48 62 71
import Pic48 from '../../public/pictures/pic48.jpg'
import Pic62 from '../../public/pictures/pic62.jpg'
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



                                                                          
<TimeToggle pageName="Исцеление Отношений, Любовь" keyName="15:40 - 15:59" validationName="Mihael" messageName="Развод" />
   

<h2 style={{
          margin: '0 0 30px'
        }}>Iahhel (Иаххель), 20:20 - 20:39</h2>
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

                                                                          
<TimeToggle pageName="Исцеление Отношений, Любовь" keyName="20:20 - 20:39" validationName="Iahhel" messageName="Развод" />
   

<h2 style={{
          margin: '0 0 30px'
        }}>Haiaiel (Хаиаиель), 23:20 - 23:39  </h2>
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

                                                                          
<TimeToggle pageName="Исцеление Отношений, Любовь" keyName="23:20 - 23:39" validationName="Haiaiel" messageName="Развод" />
   




   
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
