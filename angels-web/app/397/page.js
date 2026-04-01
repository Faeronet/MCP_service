"use client"


import { Content,Grid,Column } from "@carbon/react";
import TimeToggle from "@/components/TimeToggle/TimeToggle";

import cx from 'classnames';
import Image from 'next/image'

// 10 20 43 62
import Pic10 from '../../public/pictures/pic10.jpg'
import Pic20 from '../../public/pictures/pic20.jpg'
import Pic43 from '../../public/pictures/pic43.jpg'
import Pic62 from '../../public/pictures/pic62.jpg'



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
        }}>Aladiah (Аладиах), 03:00 - 03:19</h2>
       <div>
      <Image
        src={Pic10}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

                                    
<TimeToggle pageName="Исцеление Отношений, Брак, супружеская верность" keyName="03:00 - 03:19" validationName="Aladiah" messageName="Похоть" />




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

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

                                           
<TimeToggle pageName="Исцеление Отношений, Брак, супружеская верность" keyName="06:20 - 06:39" validationName="Pahaliah" messageName="Похоть" />



        


<h2 style={{
          margin: '0 0 30px'
        }}>Veuliah (Вевалиах), 14:00 - 14:19</h2>
       <div>
      <Image
        src={Pic43}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

                                           
<TimeToggle pageName="Исцеление Отношений, Брак, супружеская верность" keyName="14:00 - 14:19" validationName="Veuliah" messageName="Похоть" />




<h2 style={{
          margin: '0 0 30px'
        }}>Iahhel (Иаххель), 20:20 - 20:39</h2>
       <div>
      <Image
        src={Pic62}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

                                           
<TimeToggle pageName="Исцеление Отношений, Брак, супружеская верность" keyName="20:20 - 20:39" validationName="Iahhel" messageName="Похоть" />






   
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
