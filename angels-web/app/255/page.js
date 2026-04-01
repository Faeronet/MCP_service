"use client"


import { Content,Grid,Column } from "@carbon/react";
import cx from 'classnames';
import Image from 'next/image'

//7 25 30 
import Pic7 from '../../public/pictures/pic7.jpg'
import Pic25 from '../../public/pictures/pic25.jpg'
import Pic30 from '../../public/pictures/pic30.jpg'


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
        }}> Achaiah (Ахаиах) , 02:00 - 02:19 </h2>
       <div>
      <Image
        src={Pic7}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

<h2 style={{
          margin: '0 0 30px'
        }}> Nith-Haiah (Нитхаиах) ,  08:00 - 08:19 </h2>
       <div>
      <Image
        src={Pic25}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>


<h2 style={{
          margin: '0 0 30px'
        }}> Omael (Ормаёль) , 09:40 - 09:59 </h2>
       <div>
      <Image
        src={Pic30}
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
