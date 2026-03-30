"use client"


import { Content,Grid,Column } from "@carbon/react";
import cx from 'classnames';
import Image from 'next/image'

// 1 4 17 45 58 
import Pic1 from '../../public/pictures/pic1.jpg'
import Pic4 from '../../public/pictures/pic4.jpg'
import Pic17 from '../../public/pictures/pic17.jpg'
import Pic45 from '../../public/pictures/pic45.jpg'
import Pic58 from '../../public/pictures/pic58.jpg'


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
        }}> Vehuiah «Вехюиах», 00:00 - 00:19</h2>
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


<h2 style={{
          margin: '0 0 30px'
        }}> Elemiah (Элемиах), 01:00 - 01:19</h2>
       <div>
      <Image
        src={Pic4}
        alt="Example Image"
        className={styles.responsiveImage}
      />
  </div>
  <h2 style={{
          margin: '0 0 30px'
        }}></h2>


<h2 style={{
          margin: '0 0 30px'
        }}> Leviah (Левиах) , 05:20 - 05:39</h2>
       <div>
      <Image
        src={Pic17}
        alt="Example Image"
        className={styles.responsiveImage}
      />
  </div>
  <h2 style={{
          margin: '0 0 30px'
        }}></h2>


<h2 style={{
          margin: '0 0 30px'
        }}> Sealiah (Сеалиах), 14:40 - 14:59</h2>
       <div>
      <Image
        src={Pic45}
        alt="Example Image"
        className={styles.responsiveImage}
      />
  </div>
  <h2 style={{
          margin: '0 0 30px'
        }}></h2>

<h2 style={{
          margin: '0 0 30px'
        }}> Yeialel (Иеиалель), 19:00 - 19:19 </h2>
       <div>
      <Image
        src={Pic58}
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
