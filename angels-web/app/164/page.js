"use client"


import { Content,Grid,Column } from "@carbon/react";
import TimeToggle from "@/components/TimeToggle/TimeToggle";

import cx from 'classnames';
import Image from 'next/image'


import Pic9 from '../../public/pictures/pic9.jpg'
import Pic66 from '../../public/pictures/pic66.jpg'


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
        }}>Haziel (Хазиель) , 02:40 - 02:59</h2>
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

                                                 
<TimeToggle pageName="Исцеление Сознания 2, Вечный сон,спячка" keyName=" 02:40 - 02:59" validationName="Haziel" messageName="Враждебность" />


<h2 style={{
          margin: '0 0 30px'
        }}> Manakel (Манакель), 21:40 - 21:59</h2>
       <div>
      <Image
        src={Pic66}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

                                                 
<TimeToggle pageName="Исцеление Сознания 2, Вечный сон,спячка" keyName=" 21:40 - 21:59" validationName="Manakel" messageName="Враждебность" />

   
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
