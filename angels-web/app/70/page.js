"use client"


import { Content,Grid,Column } from "@carbon/react";
import TimeToggle from "@/components/TimeToggle/TimeToggle";

import cx from 'classnames';
import Image from 'next/image'


import Pic68 from '../../public/pictures/pic68.jpg'


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
        }}>Habuhiah (Хабюиах), 22:20 - 22:39</h2>
       <div>
      <Image
        src={Pic68}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

         
<TimeToggle pageName="Исцеление Тела, Органы и системы" keyName="22:20 - 22:39" validationName="Habuhiah" messageName="Слизистые оболочки" />


   
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
