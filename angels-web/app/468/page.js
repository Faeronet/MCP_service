"use client"


import { Content,Grid,Column } from "@carbon/react";
import cx from 'classnames';
import Image from 'next/image'

import styles from '../../app/case.module.css'

const BOT_URL = 'https://t.me/tet_mcp_bot#'
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
       <div
         style={{
           display: 'flex',
           flexDirection: 'column',
           alignItems: 'center',
           width: '100%',
           marginBottom: '24px'
         }}
       >
      <div style={{ width: '100%', maxWidth: 'min(100%, 420px)' }}>
      <Image
        src="/telegram-qr-tet-mcp-bot.png"
        alt="QR-код Telegram-бота @tet_mcp_bot"
        width={948}
        height={1134}
        className={styles.responsiveImage}
        priority
      />
    </div>

<div style={{ width: '100%', textAlign: 'center' }}>
     <p style={{ marginTop: '16px' }}>
            <a href={BOT_URL}>Telegram ссылка</a>.
    </p>
</div>
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
