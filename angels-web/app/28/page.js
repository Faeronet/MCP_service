"use client"


import { Content,Grid,Column } from "@carbon/react";
import TimeToggle from "@/components/TimeToggle/TimeToggle";
import cx from 'classnames';
import Image from 'next/image'


import Pic29 from '../../public/pictures/pic29.jpg'
import Pic15 from '../../public/pictures/pic15.jpg'
import Pic20 from '../../public/pictures/pic20.jpg'
import Pic41 from '../../public/pictures/pic41.jpg'
import Pic44 from '../../public/pictures/pic44.jpg'
                                                   


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
        }}>Reiyel (Рейиель), 09:20 - 09:39</h2>
       <div>
      <Image
        src={Pic29}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>

<TimeToggle pageName="Исцеление Сознания, Зависимости" keyName="09:20 - 09:39" validationName="Reiyel" messageName="Фанатизм" />

 <h2 style={{
          margin: '0 0 30px'
        }}>Hariel (Хариель), 04:40 до 04:59</h2>
       <div>
      <Image
        src={Pic15}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>
<TimeToggle pageName="Исцеление Сознания, Зависимости" keyName="04:40 до 04:59" validationName="Hariel" messageName="Фанатизм" />

 <h2 style={{
          margin: '0 0 30px'
        }}>Pahaliah (Пахалиах), 06:20 - 06:39 </h2>
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
        <TimeToggle pageName="Исцеление Сознания, Зависимости" keyName="06:20 - 06:39" validationName="Pahaliah" messageName="Фанатизм" />


 <h2 style={{
          margin: '0 0 30px'
        }}>Hahahel (Хахахель), 13:20 - 13:39</h2>
       <div>
      <Image
        src={Pic41}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

    <h2 style={{
          margin: '0 0 30px'
        }}></h2>
        <TimeToggle pageName="Исцеление Сознания, Зависимости" keyName="13:20 - 13:39" validationName="Hahahel" messageName="Фанатизм" />



 <h2 style={{
          margin: '0 0 30px'
        }}>Yelahiah (Иелахиах), 14:20 - 14:39 </h2>
       <div>
      <Image
        src={Pic44}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>
    <h2 style={{
          margin: '0 0 30px'
        }}></h2>
        <TimeToggle pageName="Исцеление Сознания, Зависимости" keyName="14:20 - 14:39" validationName="Yelahiah" messageName="Фанатизм" />






   
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
