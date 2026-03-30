"use client"


import { Content,Grid,Column } from "@carbon/react";
import cx from 'classnames';
import Image from 'next/image'

//2, 8, 14 , 22, 64 , 71 
import Pic2 from '../../public/pictures/pic2.jpg'
import Pic8 from '../../public/pictures/pic8.jpg'
import Pic14 from '../../public/pictures/pic14.jpg'
import Pic22 from '../../public/pictures/pic22.jpg'
import Pic64 from '../../public/pictures/pic64.jpg'
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
        }}> Jeliel (Иелиель), 00:20 - 00:39</h2>
       <div>
      <Image
        src={Pic2}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>


<h2 style={{
          margin: '0 0 30px'
        }}> Cahetel (Кахетель), 02:20 - 02:39 </h2>
       <div>
      <Image
        src={Pic8}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>


<h2 style={{
          margin: '0 0 30px'
        }}> Mebahel (Мебахель), 04:20 - 04:39 </h2>
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
        }}> Yeiayel (Иеиаиель), 07:40 - 07:59 </h2>
       <div>
      <Image
        src={Pic22}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

<h2 style={{
          margin: '0 0 30px'
        }}> Mehiel (Мехиель) , 21:00 - 21:19 </h2>
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
        }}> Haiaiel (Хаиаиель) , 23:20 - 23:39 </h2>
       <div>
      <Image
        src={Pic71}
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
