"use client"


import { Content,Grid,Column } from "@carbon/react";
import TimeToggle from "@/components/TimeToggle/TimeToggle";


import cx from 'classnames';
import Image from 'next/image'

// 4 13 16
import Pic4 from '../../public/pictures/pic4.jpg'
import Pic13 from '../../public/pictures/pic13.jpg'
import Pic16 from '../../public/pictures/pic16.jpg'


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
        }}>Elemiah (Элемиах),01:00 - 01:19</h2>
       <div>
      <Image
        src={Pic4}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

                   
<TimeToggle pageName="Исцеление Отношений, Брак, супружеская верность" keyName="01:00 - 01:19" validationName="Elemiah" messageName="Измена, предательство" />


<h2 style={{
          margin: '0 0 30px'
        }}>Iezalel (Иезелель), 04:00 - 04:19</h2>
       <div>
      <Image
        src={Pic13}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

                         
<TimeToggle pageName="Исцеление Отношений, Брак, супружеская верность" keyName="04:00 - 04:19" validationName="Iezalel" messageName="Измена, предательство" />


<h2 style={{
          margin: '0 0 30px'
        }}>Hekamiah (Хакамиах), 05:00 - 05:19 </h2>
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

                         
<TimeToggle pageName="Исцеление Отношений, Брак, супружеская верность" keyName="05:00 - 05:19" validationName="Hekamiah" messageName="Измена, предательство" />

  



   
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
