"use client"


import { Content,Grid,Column } from "@carbon/react";
import TimeToggle from "@/components/TimeToggle/TimeToggle";

import cx from 'classnames';
import Image from 'next/image'

// 2 33 52 70 
import Pic2 from '../../public/pictures/pic2.jpg'
import Pic33 from '../../public/pictures/pic33.jpg'
import Pic52 from '../../public/pictures/pic52.jpg'
import Pic70 from '../../public/pictures/pic70.jpg'


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
        }}>Jeliel «Иелиель»,00:20 - 00:39</h2>
       <div>
      <Image
        src={Pic2}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

                          
<TimeToggle pageName="Исцеление Отношений, Брак, супружеская верность" keyName="00:20 - 00:39" validationName="Jeliel" messageName="Конфликт, предательство, вписанный в генетический код" />


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

                                 
<TimeToggle pageName="Исцеление Отношений, Брак, супружеская верность" keyName="10:40 - 10:59" validationName="Yehuiah" messageName="Конфликт, предательство, вписанный в генетический код" />



<h2 style={{
          margin: '0 0 30px'
        }}>Imamiah (Имамиах), 17:00 - 17:19</h2>
       <div>
      <Image
        src={Pic52}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

                                 
<TimeToggle pageName="Исцеление Отношений, Брак, супружеская верность" keyName="17:00 - 17:19" validationName="Imamiah" messageName="Конфликт, предательство, вписанный в генетический код" />



<h2 style={{
          margin: '0 0 30px'
        }}>Jabamiah (Иабамиах), 23:00 - 23:19</h2>
       <div>
      <Image
        src={Pic70}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>
                                 
<TimeToggle pageName="Исцеление Отношений, Брак, супружеская верность" keyName="23:00 - 23:19" validationName="Jabamiah" messageName="Конфликт, предательство, вписанный в генетический код" />



   
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
