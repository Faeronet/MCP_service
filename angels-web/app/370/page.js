"use client"


import { Content,Grid,Column } from "@carbon/react";
import cx from 'classnames';
import Image from 'next/image'

// 5 24 32
import Pic5 from '../../public/pictures/pic5.jpg'
import Pic24 from '../../public/pictures/pic24.jpg'
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
        }}>Mahasiah (Махазиах), 01:20 - 01:39</h2>
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
        }}>Haheuiah (Хахюиах), 07:40 - 07:59</h2>
       <div>
      <Image
        src={Pic24}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

<h2 style={{
          margin: '0 0 30px'
        }}>Vasariah (Васариах), 10:20 - 10:39</h2>
       <div>
      <Image
        src={Pic32}
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
