"use client"


import { Content,Grid,Column } from "@carbon/react";
import TimeToggle from "@/components/TimeToggle/TimeToggle";

import cx from 'classnames';
import Image from 'next/image'

//1 5 33 34 39

import Pic1 from '../../public/pictures/pic1.jpg'

import Pic5 from '../../public/pictures/pic5.jpg'
import Pic33 from '../../public/pictures/pic33.jpg'
import Pic34 from '../../public/pictures/pic34.jpg'
import Pic39 from '../../public/pictures/pic39.jpg'


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
        }}> Vehuiah «Вехюиах»,  00:00 - 00:19</h2>
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

<TimeToggle pageName="Исцеление Сознания 2, Авторитаризм" keyName="00:00 - 00:19" validationName="Vehuiah" messageName="Авторитаризм" />


<h2 style={{
          margin: '0 0 30px'
        }}> Mahasiah (Махазиах), 01:20 - 01:39</h2>
       <div>
      <Image
        src={Pic5}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

<TimeToggle pageName="Исцеление Сознания 2, Авторитаризм" keyName="01:20 - 01:39" validationName="Mahasiah" messageName="Авторитаризм" />

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

<TimeToggle pageName="Исцеление Сознания 2, Авторитаризм" keyName="10:40 - 10:59" validationName="Yehuiah" messageName="Авторитаризм" />


<h2 style={{
          margin: '0 0 30px'
        }}> Lehahiah (Лехахиах), 11:00 - 11:19</h2>
       <div>
      <Image
        src={Pic34}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

<TimeToggle pageName="Исцеление Сознания 2, Авторитаризм" keyName="11:00 - 11:19" validationName="Lehahiah" messageName="Авторитаризм" />


<h2 style={{
          margin: '0 0 30px'
        }}> Rehael (Рехаёль), 12:40 - 12:59</h2>
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

        
<TimeToggle pageName="Исцеление Сознания 2, Авторитаризм" keyName="12:40 - 12:59" validationName="Rehael" messageName="Авторитаризм" />


   
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
