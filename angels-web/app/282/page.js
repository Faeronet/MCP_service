"use client"


import { Content,Grid,Column } from "@carbon/react";
import cx from 'classnames';
import Image from 'next/image'

// 18 27 
import Pic18 from '../../public/pictures/pic18.jpg'
import Pic27 from '../../public/pictures/pic27.jpg'


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
        }}> Caliel (Калиель) , 05:40 - 05:59</h2>
       <div>
      <Image
        src={Pic18}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>
   

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>


<h2 style={{
          margin: '0 0 30px'
        }}> Yerathel (Иератхель), 08:40 - 08:59</h2>
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
