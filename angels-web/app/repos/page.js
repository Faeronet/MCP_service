"use client"

import Image from 'next/image';
import Pic1 from '../../public/pic1.png';
import Pic2 from '../../public/pic2.jpg';
import TimeToggle from '@/components/TimeToggle/TimeToggle';
import { Column, Content, Grid } from "@carbon/react";
import JustBackground from '@/components/JustBackground/JustBackground';


import '@/app/repos/_repo-page.scss'

const StoryContent = ({ useResponsiveOffset = true }) => {
  // Set the column span for full width on larger screens
  const lgSpan = 16;  // Full width for lg
  const lgOffset = 0; // No offset for full width

  const content = (
    // <Grid fullWidth> {/* Enable full width grid */}
    //   <Column 
    //     sm={{ span: 4, offset: 0 }}
    //     md={{ span: 8, offset: 0 }}
    //     lg={{ span: lgSpan, offset: lgOffset }}  // Full width for lg
    //   >
    //     <div>
    //       <BackgroundWithText
    //         backgroundImage="/pic2.jpg"  // Path to the background image
    //         title="Test App"
    //         text="This text appears in front of the image background."
    //         middleImage="/pic1.png"  // Middle image passed as prop
    //         timeToggleProps={{
    //           pageName: "repos page",
    //           keyName: "time2",
    //           validationName: "Validation message for time2 on repos",
    //           messageName: "Test message for time2"
    //         }}
    //       />
    //     </div>
    //         <br/>
    //     <div>
    //       <BackgroundWithText
    //         backgroundImage="/pic2.jpg"  // Path to the background image
    //         title="Test App"
    //         text="This text appears in front of the image background."
    //         middleImage="/pic1.png"  // Middle image passed as prop
    //         timeToggleProps={{
    //           pageName: "repos page",
    //           keyName: "time2",
    //           validationName: "Validation message for time2 on repos",
    //           messageName: "Test message for time2"
    //         }}
    //       />
    //     </div>
    //   </Column>
    // </Grid>
          // JustBackground.js
    <div>

     <div>
          <JustBackground
            backgroundImage="pic2.jpg"  // Path to the background image
            title="Test App"
            text="This text appears in front of the image background."
            timeToggleProps={{
              pageName: "repos page",
              keyName: "time2",
              validationName: "Validation message for time2 on repos",
              messageName: "Test message for time2"
            }}
          />
        </div>
            <br/>


            <div>
          <JustBackground
            backgroundImage="pictures/pic3.jpg"  // Path to the background image
            title="Test App"
            text="This text appears in front of the image background."
            timeToggleProps={{
              pageName: "repos page",
              keyName: "time2",
              validationName: "Validation message for time2 on repos",
              messageName: "Test message for time2"
            }}
          />
        </div>
            <br/>

            <div>
          <JustBackground
            backgroundImage="pictures/pic4.jpg"  // Path to the background image
            title="Test App"
            text="This text appears in front of the image background."
            timeToggleProps={{
              pageName: "repos page",
              keyName: "time2",
              validationName: "Validation message for time2 on repos",
              messageName: "Test message for time2"
            }}
          />
        </div>
            <br/>

            <div>
          <JustBackground
            backgroundImage="pictures/pic5.jpg"  // Path to the background image
            title="Test App"
            text="This text appears in front of the image background."
            timeToggleProps={{
              pageName: "repos page",
              keyName: "time2",
              validationName: "Validation message for time2 on repos",
              messageName: "Test message for time2"
            }}
          />
        </div>
            <br/>

            <div>
          <JustBackground
            backgroundImage="pictures/pic6.jpg"  // Path to the background image
            title="Test App"
            text="This text appears in front of the image background."
            timeToggleProps={{
              pageName: "repos page",
              keyName: "time2",
              validationName: "Validation message for time2 on repos",
              messageName: "Test message for time2"
            }}
          />
        </div>
            <br/>

            <div>
          <JustBackground
            backgroundImage="pictures/pic7.jpg"  // Path to the background image
            title="Test App"
            text="This text appears in front of the image background."
            timeToggleProps={{
              pageName: "repos page",
              keyName: "time2",
              validationName: "Validation message for time2 on repos",
              messageName: "Test message for time2"
            }}
          />
        </div>
            <br/>

            <div>
          <JustBackground
            backgroundImage="pictures/pic8.jpg"  // Path to the background image
            title="Test App"
            text="This text appears in front of the image background."
            timeToggleProps={{
              pageName: "repos page",
              keyName: "time2",
              validationName: "Validation message for time2 on repos",
              messageName: "Test message for time2"
            }}
          />
        </div>
            <br/>


            <div>
          <JustBackground
            backgroundImage="pictures/pic9.jpg"  // Path to the background image
            title="Test App"
            text="This text appears in front of the image background."
            timeToggleProps={{
              pageName: "repos page",
              keyName: "time2",
              validationName: "Validation message for time2 on repos",
              messageName: "Test message for time2"
            }}
          />
        </div>
            <br/>

            <div>
          <JustBackground
            backgroundImage="pictures/pic10.jpg"  // Path to the background image
            title="Test App"
            text="This text appears in front of the image background."
            timeToggleProps={{
              pageName: "repos page",
              keyName: "time2",
              validationName: "Validation message for time2 on repos",
              messageName: "Test message for time2"
            }}
          />
        </div>
            <br/>


            <div>
          <JustBackground
            backgroundImage="pictures/pic11.jpg"  // Path to the background image
            title="Test App"
            text="This text appears in front of the image background."
            timeToggleProps={{
              pageName: "repos page",
              keyName: "time2",
              validationName: "Validation message for time2 on repos",
              messageName: "Test message for time2"
            }}
          />
        </div>
            <br/>

            <div>
          <JustBackground
            backgroundImage="pictures/pic12.jpg"  // Path to the background image
            title="Test App"
            text="This text appears in front of the image background."
            timeToggleProps={{
              pageName: "repos page",
              keyName: "time2",
              validationName: "Validation message for time2 on repos",
              messageName: "Test message for time2"
            }}
          />
        </div>
            <br/>

            <div>
          <JustBackground
            backgroundImage="pictures/pic13.jpg"  // Path to the background image
            title="Test App"
            text="This text appears in front of the image background."
            timeToggleProps={{
              pageName: "repos page",
              keyName: "time2",
              validationName: "Validation message for time2 on repos",
              messageName: "Test message for time2"
            }}
          />
        </div>
            <br/>


            <div>
          <JustBackground
            backgroundImage="pictures/pic14.jpg"  // Path to the background image
            title="Test App"
            text="This text appears in front of the image background."
            timeToggleProps={{
              pageName: "repos page",
              keyName: "time2",
              validationName: "Validation message for time2 on repos",
              messageName: "Test message for time2"
            }}
          />
        </div>
            <br/>

            <div>
          <JustBackground
            backgroundImage="pictures/pic15.jpg"  // Path to the background image
            title="Test App"
            text="This text appears in front of the image background."
            timeToggleProps={{
              pageName: "repos page",
              keyName: "time2",
              validationName: "Validation message for time2 on repos",
              messageName: "Test message for time2"
            }}
          />
        </div>
            <br/>

            <div>
          <JustBackground
            backgroundImage="pictures/pic16.jpg"  // Path to the background image
            title="Test App"
            text="This text appears in front of the image background."
            timeToggleProps={{
              pageName: "repos page",
              keyName: "time2",
              validationName: "Validation message for time2 on repos",
              messageName: "Test message for time2"
            }}
          />
        </div>
            <br/>

            <div>
          <JustBackground
            backgroundImage="pictures/pic17.jpg"  // Path to the background image
            title="Test App"
            text="This text appears in front of the image background."
            timeToggleProps={{
              pageName: "repos page",
              keyName: "time2",
              validationName: "Validation message for time2 on repos",
              messageName: "Test message for time2"
            }}
          />
        </div>
            <br/>


            <div>
          <JustBackground
            backgroundImage="pictures/pic18.jpg"  // Path to the background image
            title="Test App"
            text="This text appears in front of the image background."
            timeToggleProps={{
              pageName: "repos page",
              keyName: "time2",
              validationName: "Validation message for time2 on repos",
              messageName: "Test message for time2"
            }}
          />
        </div>
            <br/>

            <div>
          <JustBackground
            backgroundImage="pictures/pic19.jpg"  // Path to the background image
            title="Test App"
            text="This text appears in front of the image background."
            timeToggleProps={{
              pageName: "repos page",
              keyName: "time2",
              validationName: "Validation message for time2 on repos",
              messageName: "Test message for time2"
            }}
          />
        </div>
            <br/>

            <div>
          <JustBackground
            backgroundImage="pictures/pic20.jpg"  // Path to the background image
            title="Test App"
            text="This text appears in front of the image background."
            timeToggleProps={{
              pageName: "repos page",
              keyName: "time2",
              validationName: "Validation message for time2 on repos",
              messageName: "Test message for time2"
            }}
          />
        </div>
            <br/>


            <div>
          <JustBackground
            backgroundImage="pictures/pic21.jpg"  // Path to the background image
            title="Test App"
            text="This text appears in front of the image background."
            timeToggleProps={{
              pageName: "repos page",
              keyName: "time2",
              validationName: "Validation message for time2 on repos",
              messageName: "Test message for time2"
            }}
          />
        </div>
            <br/>


            <div>
          <JustBackground
            backgroundImage="pictures/pic22.jpg"  // Path to the background image
            title="Test App"
            text="This text appears in front of the image background."
            timeToggleProps={{
              pageName: "repos page",
              keyName: "time2",
              validationName: "Validation message for time2 on repos",
              messageName: "Test message for time2"
            }}
          />
        </div>
            <br/>


            <div>
          <JustBackground
            backgroundImage="pictures/pic23.jpg"  // Path to the background image
            title="Test App"
            text="This text appears in front of the image background."
            timeToggleProps={{
              pageName: "repos page",
              keyName: "time2",
              validationName: "Validation message for time2 on repos",
              messageName: "Test message for time2"
            }}
          />
        </div>
            <br/>

            <div>
          <JustBackground
            backgroundImage="pictures/pic24.jpg"  // Path to the background image
            title="Test App"
            text="This text appears in front of the image background."
            timeToggleProps={{
              pageName: "repos page",
              keyName: "time2",
              validationName: "Validation message for time2 on repos",
              messageName: "Test message for time2"
            }}
          />
        </div>
            <br/>


            <div>
          <JustBackground
            backgroundImage="pictures/pic25.jpg"  // Path to the background image
            title="Test App"
            text="This text appears in front of the image background."
            timeToggleProps={{
              pageName: "repos page",
              keyName: "time2",
              validationName: "Validation message for time2 on repos",
              messageName: "Test message for time2"
            }}
          />
        </div>
            <br/>

            <div>
          <JustBackground
            backgroundImage="pictures/pic26.jpg"  // Path to the background image
            title="Test App"
            text="This text appears in front of the image background."
            timeToggleProps={{
              pageName: "repos page",
              keyName: "time2",
              validationName: "Validation message for time2 on repos",
              messageName: "Test message for time2"
            }}
          />
        </div>
            <br/>

            <div>
          <JustBackground
            backgroundImage="pictures/pic27.jpg"  // Path to the background image
            title="Test App"
            text="This text appears in front of the image background."
            timeToggleProps={{
              pageName: "repos page",
              keyName: "time2",
              validationName: "Validation message for time2 on repos",
              messageName: "Test message for time2"
            }}
          />
        </div>
            <br/>

            <div>
          <JustBackground
            backgroundImage="pictures/pic28.jpg"  // Path to the background image
            title="Test App"
            text="This text appears in front of the image background."
            timeToggleProps={{
              pageName: "repos page",
              keyName: "time2",
              validationName: "Validation message for time2 on repos",
              messageName: "Test message for time2"
            }}
          />
        </div>
            <br/>

            <div>
          <JustBackground
            backgroundImage="pictures/pic29.jpg"  // Path to the background image
            title="Test App"
            text="This text appears in front of the image background."
            timeToggleProps={{
              pageName: "repos page",
              keyName: "time2",
              validationName: "Validation message for time2 on repos",
              messageName: "Test message for time2"
            }}
          />
        </div>
            <br/>


 





             







    </div>
  );

  const style = {
    height: '100%',
    margin: useResponsiveOffset ? '0' : 'auto',
    width: '100%' // Make sure content takes full width
  };

  return (
    // <Content id="main-content" style={style}>
    //   {content}
    // </Content>
    <div>
      {content}
    </div>
  );
};

export default function RepoPage() {
  return (
    <div  className="backgroundContainer">
      <StoryContent />
    </div>
  );
}
