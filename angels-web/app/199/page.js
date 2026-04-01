"use client"


import { Content,Grid,Column } from "@carbon/react";
import TimeToggle from "@/components/TimeToggle/TimeToggle";

import cx from 'classnames';
import Image from 'next/image'

//11 14 21 27 42
import Pic11 from '../../public/pictures/pic11.jpg'
import Pic14 from '../../public/pictures/pic14.jpg'
import Pic21 from '../../public/pictures/pic21.jpg'
import Pic27 from '../../public/pictures/pic27.jpg'
import Pic42 from '../../public/pictures/pic42.jpg'


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
        }}> Lauviah (Лауиах) , 03:20 - 03:39 </h2>
       <div>
      <Image
        src={Pic11}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

              
                                                                                                      
<TimeToggle pageName="Исцеление Сознания 2, Каждый сам за себя" keyName=" 03:20 - 03:39" validationName="Lauviah" messageName="Клевета" />



<h2 style={{
          margin: '0 0 30px'
        }}> Mebahel (Мебахель) , 04:20 - 04:39 </h2>
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

              
                                                                                                      
<TimeToggle pageName="Исцеление Сознания 2, Каждый сам за себя" keyName=" 04:20 - 04:39" validationName="Mebahel" messageName="Клевета" />


<h2 style={{
          margin: '0 0 30px'
        }}> Nelkhael (Нелькаель) ,  06:40 - 06:59 </h2>
       <div>
      <Image
        src={Pic21}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

              
                                                                                                      
<TimeToggle pageName="Исцеление Сознания 2, Каждый сам за себя" keyName=" 06:40 - 06:59" validationName="Nelkhael" messageName="Клевета" />


<h2 style={{
          margin: '0 0 30px'
        }}> Yerathel (Иератхель), 08:40 - 08:59</h2>
       <div>
      <Image
        src={Pic27}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

              
                                                                                                      
<TimeToggle pageName="Исцеление Сознания 2, Каждый сам за себя" keyName=" 08:40 - 08:59" validationName="Yerathel" messageName="Клевета" />


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

              
                                                                                                      
<TimeToggle pageName="Исцеление Сознания 2, Каждый сам за себя" keyName=" 13:40 - 13:59" validationName="Mikael" messageName="Клевета" />

   
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
