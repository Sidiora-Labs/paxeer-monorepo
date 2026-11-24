import { noop } from 'es-toolkit';
import React from 'react';

import { Button } from 'toolkit/chakra/button';
import { DialogActionTrigger, DialogBody, DialogContent, DialogFooter, DialogHeader, DialogRoot, DialogTrigger } from 'ui/marketplace/NativeDialog';

import { Section, Container, SectionHeader, SamplesStack, Sample } from './parts';
import { TEXT } from './utils';

const DialogShowcase = () => {

  return (
    <Container value="dialog">
      <Section>
        <SectionHeader>Size</SectionHeader>
        <SamplesStack>
          <Sample label="size: sm">
            <DialogRoot size="sm">
              <DialogTrigger asChild>
                <Button size="sm" h="auto" minH={ 8 }>
                  Open Dialog
                </Button>
              </DialogTrigger>
              <DialogContent>
                <DialogHeader>
                  Dialog Title
                </DialogHeader>
                <DialogBody>
                  <p>{ TEXT }</p>
                </DialogBody>
                <DialogFooter>
                  <Button h="auto" minH={ 10 }>Save</Button>
                  <DialogActionTrigger asChild>
                    <Button variant="link">I'll do it later</Button>
                  </DialogActionTrigger>
                </DialogFooter>
              </DialogContent>
            </DialogRoot>
          </Sample>

          <Sample label="size: md">
            <DialogRoot size="md">
              <DialogTrigger asChild>
                <Button size="sm" h="auto" minH={ 8 }>
                  Open Dialog
                </Button>
              </DialogTrigger>
              <DialogContent>
                <DialogHeader onBackToClick={ noop }>
                  Dialog Title
                </DialogHeader>
                <DialogBody>
                  <p>{ TEXT }</p>
                </DialogBody>
                <DialogFooter>
                  <DialogActionTrigger asChild>
                    <Button variant="outline" h="auto" minH={ 10 }>Cancel</Button>
                  </DialogActionTrigger>
                  <Button h="auto" minH={ 10 }>Save</Button>
                </DialogFooter>
              </DialogContent>
            </DialogRoot>
          </Sample>

          <Sample label="size: full">
            <DialogRoot size="full">
              <DialogTrigger asChild>
                <Button size="sm" h="auto" minH={ 8 }>
                  Open Dialog
                </Button>
              </DialogTrigger>
              <DialogContent>
                <DialogHeader>
                  Dialog Title
                </DialogHeader>
                <DialogBody>
                  <p>{ TEXT }</p>
                </DialogBody>
                <DialogFooter>
                  <DialogActionTrigger asChild>
                    <Button variant="outline" h="auto" minH={ 10 }>Cancel</Button>
                  </DialogActionTrigger>
                  <Button h="auto" minH={ 10 }>Save</Button>
                </DialogFooter>
              </DialogContent>
            </DialogRoot>
          </Sample>
        </SamplesStack>
      </Section>

    </Container>
  );
};

export default React.memo(DialogShowcase);
