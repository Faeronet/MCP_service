"use client"


import { Content,Grid,Column } from "@carbon/react";
import cx from 'classnames';
import Image from 'next/image'

//9 11 13 16 48 68 
import Pic9 from '../../public/pictures/pic9.jpg'
import Pic11 from '../../public/pictures/pic11.jpg'
import Pic13 from '../../public/pictures/pic13.jpg'
import Pic16 from '../../public/pictures/pic16.jpg'
import Pic48 from '../../public/pictures/pic48.jpg'
import Pic68 from '../../public/pictures/pic68.jpg'


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
        }}> Haziel (Хазиель), 02:40 - 02:59 </h2>
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


<h2 style={{
          margin: '0 0 30px'
        }}> Lauviah (Лауиах), 03:20 - 03:39</h2>
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
        }}> Iezalel (Иезелель), 04:00 - 04:19</h2>
       <div>
      <Image
        src={Pic13}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

<h2 style={{
          margin: '0 0 30px'
        }}> Hekamiah (Хакамиах), 05:00 - 05:19</h2>
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

<h2 style={{
          margin: '0 0 30px'
        }}> Mihael (Михаёль), 15:40 - 15:59 </h2>
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

<h2 style={{
          margin: '0 0 30px'
        }}> Habuhiah (Хабюиах), 22:20 - 22:39</h2>
       <div>
      <Image
        src={Pic68}
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
