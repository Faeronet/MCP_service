"use client"


import { Content,Grid,Column } from "@carbon/react";
import cx from 'classnames';
import Image from 'next/image'

// 5 13 21 27  47 53 62 
import Pic5 from '../../public/pictures/pic5.jpg'
import Pic13 from '../../public/pictures/pic13.jpg'
import Pic21 from '../../public/pictures/pic21.jpg'
import Pic27 from '../../public/pictures/pic27.jpg'
import Pic47 from '../../public/pictures/pic47.jpg'
import Pic53 from '../../public/pictures/pic53.jpg'
import Pic62 from '../../public/pictures/pic62.jpg'


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
        }}> Mahasiah (Махазиах) , 01:20 - 01:39 </h2>
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
        }}> Iezalel (Иезелель) , 04:00 - 04:19 </h2>
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
        }}> Nelkhael (Нелькаель) , 06:40 - 06:59 </h2>
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

<h2 style={{
          margin: '0 0 30px'
        }}> Yerathel (Иератхель), 08:40 - 08:59 </h2>
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

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

<h2 style={{
          margin: '0 0 30px'
        }}> Nanael (Нанаёль) , 17:20 - 17:39 </h2>
       <div>
      <Image
        src={Pic53}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

<h2 style={{
          margin: '0 0 30px'
        }}> Iahhel (Иаххель) , 20:20 - 20:39 </h2>
       <div>
      <Image
        src={Pic62}
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
