"use client"


import { Content,Grid,Column } from "@carbon/react";
import cx from 'classnames';
import Image from 'next/image'
import TimeToggle from "@/components/TimeToggle/TimeToggle";


import Pic53 from '../../public/pictures/pic53.jpg'


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
        }}>Nanael (Нанаёль), 17:20 - 17:39</h2>
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

<TimeToggle pageName="Исцеление Сознания, Неверие" keyName="17:20 - 17:39" validationName="Nanael" messageName="Обеспечивает связь с Божественым" />
   
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
