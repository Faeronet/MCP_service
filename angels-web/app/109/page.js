"use client"


import { Content,Grid,Column } from "@carbon/react";
import TimeToggle from "@/components/TimeToggle/TimeToggle";

import cx from 'classnames';
import Image from 'next/image'


import Pic50 from '../../public/pictures/pic50.jpg'
import Pic60 from '../../public/pictures/pic60.jpg'
import Pic1 from '../../public/pictures/pic1.jpg'
import Pic14 from '../../public/pictures/pic14.jpg'


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
        }}>Vehuiah «Вехюиах», 00:00 - 00:19</h2>
       <div>
      <Image
        src={Pic1}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

            
    <TimeToggle pageName="Исцеление Тела, Нервозность,Ожирение" keyName="00:00 - 00:19" validationName="Vehuiah" messageName="Нервозность, нервное напряжение, возбудимость" />
   



  <h2 style={{
          margin: '0 0 30px'
        }}>Mebahel (Мебахель), 04:20 - 04:39</h2>
       <div>
      <Image
        src={Pic14}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

            
    <TimeToggle pageName="Исцеление Тела, Нервозность,Ожирение" keyName=" 04:20 - 04:39" validationName="Mebahel" messageName="Нервозность, нервное напряжение, возбудимость" />
   


  <h2 style={{
          margin: '0 0 30px'
        }}>Daniel (Даниель), 16:20 - 16:39</h2>
       <div>
      <Image
        src={Pic50}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

            
    <TimeToggle pageName="Исцеление Тела,Нервозность,Ожирение" keyName="16:20 - 16:39" validationName="Daniel" messageName="Нервозность, нервное напряжение, возбудимость" />
   

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

            
    <TimeToggle pageName="Исцеление Тела,Нервозность,Ожирение" keyName="19:40 - 19:59" validationName="Mitzrael" messageName="Нервозность, нервное напряжение, возбудимость" />
   
   
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
