import React, { useState, useEffect } from 'react';
import { Toggle, TimePicker, Button, InlineNotification } from '@carbon/react';
import { Save } from '@carbon/icons-react';

const hashCode = (str) => {
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    const char = str.charCodeAt(i);
    hash = (hash << 5) - hash + char;
    hash |= 0; // Convert to 32bit integer
  }
  return hash.toString();
};

const TimeToggle = ({ pageName, keyName, validationName, messageName }) => {
  const [showTime, setShowTime] = useState(false);
  const [timeValue, setTimeValue] = useState('');
  const [validationError, setValidationError] = useState('');

  const storageKey = hashCode(`${pageName}-${keyName}-${validationName}-${messageName}`);

  useEffect(() => {
    // Load the saved state from localStorage
    const savedData = JSON.parse(localStorage.getItem('timeData')) || {};
    if (savedData[storageKey]) {
      setShowTime(savedData[storageKey].show);
      setTimeValue(savedData[storageKey].value);
    }
  }, [storageKey]);

  const saveToLocalStorage = (data) => {
    try {
      let savedData = JSON.parse(localStorage.getItem('timeData')) || {};

      savedData[storageKey] = {
        ...savedData[storageKey],
        ...data,
        часть: pageName,
        keyName,   // Save keyName
      };

      localStorage.setItem('timeData', JSON.stringify(savedData));
      console.log('Data saved successfully to local storage!');
    } catch (error) {
      console.error('Error saving to local storage:', error);
    }
  };

  const removeFromLocalStorage = () => {
    try {
      let savedData = JSON.parse(localStorage.getItem('timeData')) || {};
      if (savedData[storageKey]) {
        delete savedData[storageKey];
        localStorage.setItem('timeData', JSON.stringify(savedData));
        console.log('Data removed from local storage!');
      }
    } catch (error) {
      console.error('Error removing from local storage:', error);
    }
  };

  const handleToggle = () => {
    const newShowTime = !showTime;
    setShowTime(newShowTime);

    if (!newShowTime) {
      removeFromLocalStorage();
      setTimeValue(''); // Reset time value when toggled off
    }
  };

  const validateTime = (time) => {
    if (time.length !== 5) {
      return 'Invalid time format. Please use HH:MM.';
    }

    const timePattern = /^([01]\d|2[0-3]):([0-5]\d)$/;
    if (!timePattern.test(time)) {
      return 'Invalid time format. Please use HH:MM.';
    }

    return '';
  };

  const handleSave = () => {
    const error = validateTime(timeValue);
    if (error) {
      setValidationError(error);
      return;
    }

    const validation = validationName;
    const цель = messageName;

    const data = {
      show: showTime,
      value: timeValue,
      validation,
      цель,
    };

    saveToLocalStorage(data);
    setValidationError(''); // Clear any previous validation errors
  };

  return (
    <div style={{ display: 'flex', justifyContent: 'center', marginBottom: '20px' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: '20px' }}>
        <Toggle
          id={`toggle-${keyName}`}
          onToggle={handleToggle}
          toggled={showTime}
        />
        {showTime && (
          <div style={{ display: 'flex', alignItems: 'center', gap: '20px' }}>
            <TimePicker
              id={`time-picker-${keyName}`}
              value={timeValue}
              onChange={(event) => setTimeValue(event.target.value)}
            />
            <div>
              <Button
                onClick={handleSave}
                size="sm"
                renderIcon={Save}
                iconDescription="Save"
                hasIconOnly
                kind="primary"
                tooltipAlignment="center"
                tooltipPosition="bottom"
              />
            </div>
            {validationError && (
              <InlineNotification
                kind="error"
                title="Validation Error"
                subtitle={validationError}
                onClose={() => setValidationError('')}
              />
            )}
          </div>
        )}
      </div>
    </div>
  );
};

export default TimeToggle;
