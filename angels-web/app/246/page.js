"use client"


import { Content,Grid,Column } from "@carbon/react";
import cx from 'classnames';
import Image from 'next/image'

//9 39 41 49 

import Pic9 from '../../public/pictures/pic9.jpg'
import Pic39 from '../../public/pictures/pic39.jpg'
import Pic41 from '../../public/pictures/pic41.jpg'
import Pic49 from '../../public/pictures/pic49.jpg'



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
        }}> Hahahel (Хахахель) , 13:20 - 13:39 </h2>
       <div>
      <Image
        src={Pic41}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

<h2 style={{
          margin: '0 0 30px'
        }}> Vehuel (Вехюель) , 16:00 - 16:19 </h2>
       <div>
      <Image
        src={Pic49}
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
