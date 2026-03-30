"use client"


import { Content,Grid,Column } from "@carbon/react";
import TimeToggle from "@/components/TimeToggle/TimeToggle";
import cx from 'classnames';
import Image from 'next/image'


import Pic19 from '../../public/pictures/pic19.jpg'
import Pic20 from '../../public/pictures/pic20.jpg'
import Pic30 from '../../public/pictures/pic30.jpg'
import Pic48 from '../../public/pictures/pic48.jpg'
import Pic59 from '../../public/pictures/pic59.jpg'
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
        }}>Leuviah (Леувиах), 06:00 - 06:19</h2>
       <div>
      <Image
        src={Pic19}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>
    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

        
<TimeToggle pageName="Исцеление Тела, Женское" keyName="06:00 - 06:19" validationName="Leuviah" messageName="Бесплодие" />




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

        
<TimeToggle pageName="Исцеление Тела, Женское" keyName="06:20 - 06:39" validationName="Pahaliah" messageName="Бесплодие" />



<h2 style={{
          margin: '0 0 30px'
        }}>Omael (Ормаёль), 09:40 - 09:59</h2>
       <div>
      <Image
        src={Pic30}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>
    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

        
<TimeToggle pageName="Исцеление Тела, Женское" keyName="09:40 - 09:59" validationName="Omael" messageName="Бесплодие" />



<h2 style={{
          margin: '0 0 30px'
        }}>Mihael (Михаёль), 15:40 - 15:59</h2>
       <div>
      <Image
        src={Pic48}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>
    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

        
<TimeToggle pageName="Исцеление Тела, Женское" keyName="15:40 - 15:59" validationName="Mihael" messageName="Бесплодие" />



<h2 style={{
          margin: '0 0 30px'
        }}>Harahel (Харахель), 19:20 - 19:39</h2>
       <div>
      <Image
        src={Pic59}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>
        
<TimeToggle pageName="Исцеление Тела, Женское" keyName="19:20 - 19:39" validationName="Harahel" messageName="Бесплодие" />


<h2 style={{
          margin: '0 0 30px'
        }}> Habuhiah (Хабюиах), 22:20 - 22:39</h2>
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

               
<TimeToggle pageName="Исцеление Тела, Женское" keyName="22:20 - 22:39" validationName="Habuhiah" messageName="Бесплодие" />



   
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
