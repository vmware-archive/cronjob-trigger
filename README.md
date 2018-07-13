# cronjob-trigger

A Kubeless _Trigger_ represents an event source that a Kubeless function can be associated with it. When an event occurs in the event source, Kubeless will ensure that the associated functions are invoked. __CronJob-trigger__ addon to Kubeless adds support for deploying functions that should be triggered following a certain schedule

Please refer to the [documentation](https://github.com/kubeless/kubeless/blob/master/docs/kubeless-functions.md#scheduled-functions) on how to use CronJob triggers with Kubeless.