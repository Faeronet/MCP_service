"use client"


import { Content,Grid,Column } from "@carbon/react";
import cx from 'classnames';
import Image from 'next/image'

// 1 4 36 45 60 64 72 
import Pic1 from '../../public/pictures/pic1.jpg'
import Pic4 from '../../public/pictures/pic4.jpg'
import Pic36 from '../../public/pictures/pic36.jpg'
import Pic45 from '../../public/pictures/pic45.jpg'
import Pic60 from '../../public/pictures/pic60.jpg'
import Pic64 from '../../public/pictures/pic64.jpg'
import Pic72 from '../../public/pictures/pic72.jpg'


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
        }}> Vehuiah (Вехюиах), 00:00 - 00:19</h2>
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
        }}> Menadel (Манадель) , 11:40 - 11:59 </h2>
       <div>
      <Image
        src={Pic36}
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
        }}> Mitzrael (Мизраель), 19:40 - 19:59</h2>
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

<h2 style={{
          margin: '0 0 30px'
        }}> Mehiel (Мехиель), 21:00 - 21:19 </h2>
       <div>
      <Image
        src={Pic64}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    
    <h2 style={{
          margin: '0 0 30px'
        }}></h2>


<h2 style={{
          margin: '0 0 30px'
        }}> Mumiah (Мюмиах), 23:40 - 23:59 </h2>
       <div>
      <Image
        src={Pic72}
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
