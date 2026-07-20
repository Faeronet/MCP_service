'use client';

import { Column, Grid } from '@carbon/react';
import AboutUserGuide from './AboutUserGuide';
import styles from './repo1.module.scss';

export default function AboutProjectPage() {
  return (
    <div className={styles.backgroundContainer1}>
      <div className={styles.pageWrap}>
        <Grid>
          <Column
            sm={{ span: 4, offset: 0 }}
            md={{ span: 8, offset: 0 }}
            lg={{ span: 12, offset: 2 }}
          >
            <AboutUserGuide />
          </Column>
        </Grid>
      </div>
    </div>
  );
}
