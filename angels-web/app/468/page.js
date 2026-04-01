"use client"


import { Content,Grid,Column } from "@carbon/react";
import cx from 'classnames';
import Image from 'next/image'


import Pic1 from '../../public/pictures/telegram.jpg'


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
        }}>Telegram</h2>
       <div>
      <Image
        src={Pic1}
        alt="Example Image"
        className={styles.responsiveImage}
      />
    </div>

<div>
     <p>
            
            <a href="https://t.me/angeliroad">Telegram ссылка</a>. 
    </p>
</div>   

<h2 style={{
          margin: '0 0 30px'
        }}>Мобильная версия для android</h2>
        <div>
          <p>

            <a href="https://drive.google.com/drive/folders/1O7hGVtX_SUmk7DsRZ4bgTVD3IzthRlkr?usp=sharing">Android ссылка</a>.
          </p>
        </div>

      




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
