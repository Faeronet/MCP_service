"use client"


import { Content,Grid,Column } from "@carbon/react";
import cx from 'classnames';
import Image from 'next/image'

//4 12 13 16 26 34 42 57 58 71

import Pic4 from '../../public/pictures/pic4.jpg'
import Pic12 from '../../public/pictures/pic12.jpg'
import Pic13 from '../../public/pictures/pic13.jpg'
import Pic16 from '../../public/pictures/pic16.jpg'
import Pic26 from '../../public/pictures/pic26.jpg'
import Pic34 from '../../public/pictures/pic34.jpg'
import Pic42 from '../../public/pictures/pic42.jpg'
import Pic57 from '../../public/pictures/pic57.jpg'
import Pic58 from '../../public/pictures/pic58.jpg'
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
        }}> Hahaiah (Хахаиах), 03:40 - 03:59</h2>
       <div>
      <Image
        src={Pic12}
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
        }}> Haaiah (Хааиах), 08:20 - 08:39</h2>
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


<h2 style={{
          margin: '0 0 30px'
        }}> Nemamiah (Неммамиах), 18:40 - 18:59</h2>
       <div>
      <Image
        src={Pic57}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>


    <h2 style={{
          margin: '0 0 30px'
        }}></h2>



<h2 style={{
          margin: '0 0 30px'
        }}> Yeialel (Иеиалель), 19:00 - 19:19</h2>
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


<h2 style={{
          margin: '0 0 30px'
        }}> Haiaiel (Хаиаиель), 23:20 - 23:39</h2>
       <div>
      <Image
        src={Pic71}
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
