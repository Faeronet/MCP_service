"use client"


import { Content,Grid,Column } from "@carbon/react";
import cx from 'classnames';
import Image from 'next/image'

//11 14 18 32 
import Pic11 from '../../public/pictures/pic11.jpg'
import Pic14 from '../../public/pictures/pic14.jpg'
import Pic18 from '../../public/pictures/pic18.jpg'
import Pic32 from '../../public/pictures/pic32.jpg'


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


<h2 style={{
          margin: '0 0 30px'
        }}> Caliel (Калиель), 05:40 - 05:59</h2>
       <div>
      <Image
        src={Pic18}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>


<h2 style={{
          margin: '0 0 30px'
        }}> Vasariah (Васариах) , 10:20 - 10:39 </h2>
       <div>
      <Image
        src={Pic32}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>
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
