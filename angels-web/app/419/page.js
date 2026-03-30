"use client"


import { Content,Grid,Column } from "@carbon/react";
import TimeToggle from "@/components/TimeToggle/TimeToggle";


import cx from 'classnames';
import Image from 'next/image'

// 12 33 40 
import Pic12 from '../../public/pictures/pic12.jpg'
import Pic33 from '../../public/pictures/pic33.jpg'
import Pic40 from '../../public/pictures/pic40.jpg'


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
        }}>Hahaiah (Хахаиах), 03:40 - 03:59</h2>
       <div>
      <Image
        src={Pic12}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

                                                     
<TimeToggle pageName="Исцеление Отношений, Любовь" keyName="03:40 - 03:59" validationName="Hahahel" messageName="Благоприятствует любви" />
   
   

<h2 style={{
          margin: '0 0 30px'
        }}>Yehuiah (Иехюиах), 10:40 - 10:59</h2>
       <div>
      <Image
        src={Pic33}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

                                                     
<TimeToggle pageName="Исцеление Отношений, Любовь" keyName="10:40 - 10:59" validationName="Yehuiah" messageName="Благоприятствует любви" />
   
   

<h2 style={{
          margin: '0 0 30px'
        }}>Ieiazel (Иейазель), 13:00 - 13:19</h2>
       <div>
      <Image
        src={Pic40}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

                                                     
<TimeToggle pageName="Исцеление Отношений, Любовь" keyName="13:00 - 13:19" validationName="Ieiazel" messageName="Благоприятствует любви" />
   
   
   
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
