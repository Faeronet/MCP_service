"use client"


import { Content,Grid,Column } from "@carbon/react";
import TimeToggle from "@/components/TimeToggle/TimeToggle";

import cx from 'classnames';
import Image from 'next/image'

// 26 42 54
import Pic26 from '../../public/pictures/pic26.jpg'
import Pic42 from '../../public/pictures/pic42.jpg'
import Pic54 from '../../public/pictures/pic54.jpg'

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
        }}> Haaiah (Хааиах) , 08:20 - 08:39</h2>
       <div>
      <Image
        src={Pic26}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

        
                                                                                        
<TimeToggle pageName="Исцеление Сознания 2, Зависть" keyName="  08:20 - 08:39" validationName="Haaiah" messageName="Заговор" />


<h2 style={{
          margin: '0 0 30px'
        }}> Mikael (Микаэль), 13:40 - 13:59</h2>
       <div>
      <Image
        src={Pic42}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

        
                                                                                        
<TimeToggle pageName="Исцеление Сознания 2, Зависть" keyName="  13:40 - 13:59" validationName="Mikael" messageName="Заговор" />


<h2 style={{
          margin: '0 0 30px'
        }}> Nithael (Нитаёль) , 17:40 - 17:59</h2>
       <div>
      <Image
        src={Pic54}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

        
                                                                                        
<TimeToggle pageName="Исцеление Сознания 2, Зависть" keyName="  17:40 - 17:59" validationName="Nithael" messageName="Заговор" />


       
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
