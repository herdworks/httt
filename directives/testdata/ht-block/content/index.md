---
layout: ok.html
title: ht-block test
---

Hello ht-block world. :wave:

<!-- This block will be wrapped in a p tag -->
<uni-form ht-block action='https://hypertexting.community/signup' submit='Join the TestFlight'></uni-form>

<!-- This block will not be wrapped in a p tag -->
<uni-form ht-block action='https://hypertexting.community/signup' submit='Join the TestFlight'>
</uni-form>

<!-- Multi-line opening tag: ht-block on a continuation line must still be detected -->
<uni-form
    ht-block
    action='https://hypertexting.community/signup'
    submit='Join the TestFlight'
></uni-form>

See <icon-thing ht-block></icon-thing> inline in prose — this should remain in a paragraph.
