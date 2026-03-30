"use client"


import { Content,Grid,Column } from "@carbon/react";
import cx from 'classnames';
import Image from 'next/image'

//12, 17 , 28 , 39 , 67 
import Pic12 from '../../public/pictures/pic12.jpg'
import Pic17 from '../../public/pictures/pic17.jpg'
import Pic28 from '../../public/pictures/pic28.jpg'
import Pic39 from '../../public/pictures/pic39.jpg'
import Pic67 from '../../public/pictures/pic67.jpg'


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
        }}> Hahaiah (Хахаиах), 03:40 - 03:59 </h2>
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
        }}> Leviah (Левиах) , 05:20 - 05:39 </h2>
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
        }}> Seheiah (Сехиах), 09:00 - 09:19 </h2>
       <div>
      <Image
        src={Pic28}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

<h2 style={{
          margin: '0 0 30px'
        }}> Rehael (Рехаёль), 12:40 - 12:59 </h2>
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

<h2 style={{
          margin: '0 0 30px'
        }}> Eyael (Эйаёль) , 22:00 - 22:19 </h2>
       <div>
      <Image
        src={Pic67}
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
