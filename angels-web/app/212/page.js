"use client"


import { Content,Grid,Column } from "@carbon/react";
import cx from 'classnames';
import Image from 'next/image'

// 16  64
import Pic16 from '../../public/pictures/pic16.jpg'
import Pic64 from '../../public/pictures/pic64.jpg'


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
        }}> Hekamiah (Хакамиах) , 05:00 - 05:19</h2>
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
        }}> Mehiel (Мехиель) , 21:00 - 21:19</h2>
       <div>
      <Image
        src={Pic64}
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
