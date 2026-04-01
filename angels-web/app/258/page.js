"use client"


import { Content,Grid,Column } from "@carbon/react";
import cx from 'classnames';
import Image from 'next/image'

//5 31 47 
import Pic5 from '../../public/pictures/pic5.jpg'
import Pic31 from '../../public/pictures/pic31.jpg'
import Pic47 from '../../public/pictures/pic47.jpg'


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
        }}> Mahasiah (Махазиах), 01:20 - 01:39 </h2>
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

<h2 style={{
          margin: '0 0 30px'
        }}> Lecabel (Лекабель) , 10:00 - 10:19 </h2>
       <div>
      <Image
        src={Pic31}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>
    <h2 style={{
          margin: '0 0 30px'
        }}></h2>


<h2 style={{
          margin: '0 0 30px'
        }}> Asaliah (Асалиах) , 15:20 - 15:39 </h2>
       <div>
      <Image
        src={Pic47}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>
   
   
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
