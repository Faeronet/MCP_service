"use client"


import { Content,Grid,Column } from "@carbon/react";
import TimeToggle from "@/components/TimeToggle/TimeToggle";

import cx from 'classnames';
import Image from 'next/image'


import Pic1 from '../../public/pictures/pic1.jpg'
import Pic3 from '../../public/pictures/pic3.jpg'
import Pic45 from '../../public/pictures/pic45.jpg'
import Pic23 from '../../public/pictures/pic23.jpg'


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
        }}>Vehuiah «Вехюиах», 00:00 - 00:19</h2>
       <div>
      <Image
        src={Pic1}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

        
                       
<TimeToggle pageName="Исцеление Тела,Раны,Инфекции" keyName="00:00 - 00:19" validationName="Vehuiah" messageName="Растяжение" />
   


 <h2 style={{
          margin: '0 0 30px'
        }}>Sitael «Ситаель», 00:40 - 00:59</h2>
       <div>
      <Image
        src={Pic3}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>
    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

        
                       
<TimeToggle pageName="Исцеление Тела,Раны,Инфекции" keyName="00:40 - 00:59" validationName="Sitael" messageName="Растяжение" />
   



 <h2 style={{
          margin: '0 0 30px'
        }}>Melahel (Мелахель), 07:20 - 07:39</h2>
       <div>
      <Image
        src={Pic23}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>
    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

        
                       
<TimeToggle pageName="Исцеление Тела,Раны,Инфекции" keyName="07:20 - 07:39" validationName="Melahel" messageName="Растяжение" />
   


 <h2 style={{
          margin: '0 0 30px'
        }}>Sealiah (Сеалиах), 14:40 - 14:59</h2>
       <div>
      <Image
        src={Pic45}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>
    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

        
                       
<TimeToggle pageName="Исцеление Тела,Раны,Инфекции" keyName="14:40 - 14:59" validationName="Sealiah" messageName="Растяжение" />
   



   
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
