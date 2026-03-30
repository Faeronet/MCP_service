"use client"


import { Content,Grid,Column } from "@carbon/react";
import TimeToggle from "@/components/TimeToggle/TimeToggle";
import cx from 'classnames';
import Image from 'next/image'


import Pic5 from '../../public/pictures/pic5.jpg'
import Pic20 from '../../public/pictures/pic20.jpg'
import Pic61 from '../../public/pictures/pic61.jpg'


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





<TimeToggle pageName="Исцеление Сознания, Зависимости" keyName="01:20 - 01:39" validationName="Mahasiah" messageName="Разврат, проституция" />

    <h2 style={{
          margin: '0 0 30px'
        }}>Pahaliah (Пахалиах), 06:20 - 06:39</h2>
       <div>
      <Image
        src={Pic20}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

  


<TimeToggle pageName="Исцеление Сознания, Зависимости" keyName="06:20 - 06:39" validationName="Pahaliah" messageName="Разврат, проституция" />

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

    <h2 style={{
          margin: '0 0 30px'
        }}>Umabel (Умабель), 20:00 - 20:19</h2>
       <div>
      <Image
        src={Pic61}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>



<TimeToggle pageName="Исцеление Сознания, Зависимости" keyName="20:00 - 20:19" validationName="Umabel" messageName="Разврат, проституция" />
   
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
